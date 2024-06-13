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

	cl := client.Client{
		URL: conf.URL,
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
		status, ok, err := checker.HealthStatus(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "%v", err)
			return
		}
		var code int
		if ok {
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
