package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"flag"

	"github.com/ecadlabs/gotez/v2/client"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v3"
)

const (
	defaultListen         = ":8080"
	defaultTimeout        = 30 * time.Second
	defaultTolerance      = 1 * time.Second
	defaultReconnectDelay = 10 * time.Second
)

type debugLogger log.Logger

func (l *debugLogger) Printf(format string, a ...any) {
	(*log.Logger)(l).Debugf(format, a...)
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
		Listen:            defaultListen,
		Timeout:           defaultTimeout,
		Tolerance:         defaultTolerance,
		ReconnectDelay:    defaultReconnectDelay,
		CheckBlockDelay:   true,
		CheckBootstrapped: true,
		CheckSyncState:    true,
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

	cl := client.Client{
		URL:         conf.URL,
		DebugLogger: (*debugLogger)(log.StandardLogger()),
	}

	mon := HeadMonitor{
		Client:         &cl,
		ChainID:        conf.ChainID,
		Timeout:        conf.Timeout,
		Tolerance:      conf.Tolerance,
		ReconnectDelay: conf.ReconnectDelay,
		UseTimestamps:  conf.UseTimestamps,
	}
	checker := HealthChecker{
		Monitor:           &mon,
		Client:            &cl,
		ChainID:           conf.ChainID,
		Timeout:           conf.Timeout,
		CheckBlockDelay:   conf.CheckBlockDelay,
		CheckBootstrapped: conf.CheckBootstrapped,
		CheckSyncState:    conf.CheckSyncState,
	}
	if conf.CheckBlockDelay {
		mon.Start()
		defer mon.Stop(context.Background())
	}

	r := mux.NewRouter()
	r.Methods("GET").Path("/health").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status, err := checker.HealthStatus(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "%v", err)
			return
		}
		var code int
		if status.IsOk() {
			code = http.StatusOK
		} else {
			code = http.StatusInternalServerError
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(status)
	})
	r.Methods("GET").Path("/sync_status").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status, err := checker.HealthStatus(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "%v", err)
			return
		}
		var code int
		if status.IsBootstrapped && status.IsSynced {
			code = http.StatusOK
		} else {
			code = http.StatusInternalServerError
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(status)
	})
	r.Methods("GET").Path("/block_delay").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status, err := checker.HealthStatus(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "%v", err)
			return
		}
		var code int
		if status.BlockDelayOk {
			code = http.StatusOK
		} else {
			code = http.StatusInternalServerError
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(status)
	})
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
