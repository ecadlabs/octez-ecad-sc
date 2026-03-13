package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"time"

	"flag"

	"github.com/ecadlabs/gotez/v2"
	client "github.com/ecadlabs/gotez/v2/clientv2"
	"github.com/ecadlabs/gotez/v2/clientv2/utils"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v3"
)

const (
	defaultListen         = ":8080"
	defaultTimeout        = 30 * time.Second
	defaultTolerance      = 1 * time.Second
	defaultReconnectDelay = 10 * time.Second
	defaultPollInterval   = 15 * time.Second
)

type debugLogger log.Logger

func (l *debugLogger) Printf(format string, a ...any) {
	(*log.Logger)(l).Debugf(format, a...)
}

type nodeInstance struct {
	name   string
	hmon   *HeadMonitor
	poller *Poller
	mmon   *MempoolMonitor
}

func main() {
	logLevel := flag.String("l", "info", "Log level: [error, warn, info, debug, trace]")
	confPath := flag.String("c", "", "Config file path")
	flag.Parse()

	ll, err := log.ParseLevel(*logLevel)
	if err != nil {
		log.Fatal(err)
	}
	log.SetLevel(ll)

	conf := Config{
		Listen:                defaultListen,
		Timeout:               defaultTimeout,
		Tolerance:             defaultTolerance,
		ReconnectDelay:        defaultReconnectDelay,
		HealthUseBlockDelay:   true,
		HealthUseBootstrapped: true,
		PollInterval:          defaultPollInterval,
	}

	buf, err := os.ReadFile(*confPath)
	if err != nil {
		log.Fatal(err)
	}
	if err := yaml.Unmarshal(buf, &conf); err != nil {
		log.Fatal(err)
	}
	tmp, _ := json.MarshalIndent(&conf, "", "    ")
	log.Info(string(tmp))

	// Build effective node list. Prefer nodes[], fall back to legacy url field.
	nodes := conf.Nodes
	if len(nodes) == 0 {
		if conf.URL == "" {
			log.Fatal("no nodes configured: set 'nodes' or 'url' in config")
		}
		nodes = []NodeConfig{{Name: "node", URL: conf.URL}}
	}

	reg := prometheus.NewRegistry()
	var instances []nodeInstance

	for _, nodeCfg := range nodes {
		// Each node's metrics carry a constant "node" label so they are
		// distinguishable in Prometheus while sharing the same registry.
		nodeReg := prometheus.WrapRegistererWith(prometheus.Labels{"node": nodeCfg.Name}, reg)

		cl := client.Client{
			URL:         nodeCfg.URL,
			DebugLogger: (*debugLogger)(log.StandardLogger()),
		}

		hmon, err := (&HeadMonitorConfig{
			Client:         &cl,
			ChainID:        conf.ChainID,
			Timeout:        conf.Timeout,
			Tolerance:      conf.Tolerance,
			ReconnectDelay: conf.ReconnectDelay,
			UseTimestamps:  conf.UseTimestamps,
			Reg:            nodeReg,
		}).New(context.Background())
		if err != nil {
			log.Fatal(err)
		}

		// Capture hmon per loop iteration to avoid closure capture issues.
		capturedHmon := hmon
		nextProto := func() *gotez.ProtocolHash { _, p := capturedHmon.Protocols(); return p }

		mmon := (&MempoolMonitorConfig{
			Client:           &cl,
			ChainID:          conf.ChainID,
			Timeout:          conf.Timeout,
			ReconnectDelay:   conf.ReconnectDelay,
			Reg:              nodeReg,
			NextProtocolFunc: nextProto,
		}).New()

		poller := (&PollerConfig{
			Client:           &cl,
			ChainID:          conf.ChainID,
			Timeout:          conf.Timeout,
			Interval:         conf.PollInterval,
			Reg:              nodeReg,
			NextProtocolFunc: nextProto,
		}).New()

		instances = append(instances, nodeInstance{
			name:   nodeCfg.Name,
			hmon:   hmon,
			poller: poller,
			mmon:   mmon,
		})
	}

	for _, inst := range instances {
		inst.hmon.Start()
		defer inst.hmon.Stop(context.Background())

		inst.poller.Start()
		defer inst.poller.Stop(context.Background())

		inst.mmon.Start()
		defer inst.mmon.Stop(context.Background())
	}

	r := mux.NewRouter()

	// /health: 200 if at least one node is healthy.
	r.Methods("GET").Path("/health").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		anyHealthy := false
		for _, inst := range instances {
			ok := true
			if conf.HealthUseBootstrapped {
				s := inst.poller.Status()
				ok = ok && s.Bootstrapped && s.SyncState == utils.SyncStateSynced
			}
			if conf.HealthUseBlockDelay {
				ok = ok && inst.hmon.Status()
			}
			if ok {
				anyHealthy = true
				break
			}
		}
		code := http.StatusInternalServerError
		if anyHealthy {
			code = http.StatusOK
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(anyHealthy)
	})

	// /sync_status: per-node bootstrap/sync state map; 200 if any node is healthy.
	r.Methods("GET").Path("/sync_status").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type nodeStatus struct {
			Status  utils.BootstrappedResponse `json:"status"`
			Healthy bool                       `json:"healthy"`
		}
		result := make(map[string]nodeStatus, len(instances))
		anyHealthy := false
		for _, inst := range instances {
			s := inst.poller.Status()
			healthy := s.Bootstrapped && s.SyncState == utils.SyncStateSynced
			if healthy {
				anyHealthy = true
			}
			result[inst.name] = nodeStatus{Status: s, Healthy: healthy}
		}
		code := http.StatusInternalServerError
		if anyHealthy {
			code = http.StatusOK
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(result)
	})

	// /block_delay: per-node block delay status map; 200 if any node is within tolerance.
	r.Methods("GET").Path("/block_delay").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := make(map[string]bool, len(instances))
		anyHealthy := false
		for _, inst := range instances {
			ok := inst.hmon.Status()
			result[inst.name] = ok
			if ok {
				anyHealthy = true
			}
		}
		code := http.StatusInternalServerError
		if anyHealthy {
			code = http.StatusOK
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(result)
	})

	r.Methods("GET").Path("/metrics").Handler(promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))
	r.Use((&Logging{}).Handler)

	srv := &http.Server{
		Handler: r,
		Addr:    conf.Listen,
	}
	go func() {
		log.Infof("Listening on %s", conf.Listen)
		srv.ListenAndServe()
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, unix.SIGINT, unix.SIGTERM)
	<-c

	srv.Shutdown(context.Background())
}
