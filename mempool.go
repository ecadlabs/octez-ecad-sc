package main

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	tz "github.com/ecadlabs/gotez/v2"
	client "github.com/ecadlabs/gotez/v2/clientv2"
	"github.com/ecadlabs/gotez/v2/clientv2/mempool"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

type MempoolMonitorConfig struct {
	Client           *client.Client
	ChainID          *tz.ChainID
	Timeout          time.Duration
	ReconnectDelay   time.Duration
	Reg              prometheus.Registerer
	NextProtocolFunc func() *tz.ProtocolHash
}

func (c *MempoolMonitorConfig) New() *MempoolMonitor {
	m := &MempoolMonitor{
		cfg: *c,
		metric: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "tezos",
			Subsystem: "node",
			Name:      "mempool_operations_total",
			Help:      "The total number of mempool operations. Resets on reconnection.",
		}, []string{"kind", "proto"}),
	}
	if c.Reg != nil {
		c.Reg.MustRegister(m.metric)
	}
	return m
}

type MempoolMonitor struct {
	cfg    MempoolMonitorConfig
	cancel context.CancelFunc
	done   chan struct{}
	metric *prometheus.CounterVec
}

func (h *MempoolMonitor) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	h.done = make(chan struct{})
	go h.serve(ctx)
}

func (h *MempoolMonitor) Stop(ctx context.Context) error {
	h.cancel()
	select {
	case <-h.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *MempoolMonitor) serve(ctx context.Context) {
	defer close(h.done)
	var err error
	for {
		if err != nil {
			log.Error(err)
			t := time.After(h.cfg.ReconnectDelay)
			select {
			case <-t:
			case <-ctx.Done():
				return
			}
		}

		h.metric.Reset()

		var (
			stream <-chan *mempool.MonitorResponse
			errCh  <-chan error
		)
		stream, errCh, err = mempool.Monitor(ctx, h.cfg.Client, h.cfg.ChainID)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			continue
		}

	Recv:
		for {
			select {
			case err = <-errCh:
				if errors.Is(err, context.Canceled) {
					return
				}
				break Recv

			case resp := <-stream:
				counter := h.metric.MustCurryWith(prometheus.Labels{"proto": h.cfg.NextProtocolFunc().String()})
				if log.GetLevel() >= log.DebugLevel {
					buf, _ := json.MarshalIndent(resp.Contents, "", "    ")
					log.Debug(string(buf))
				}

				for _, list := range resp.Contents {
					for _, grp := range list.Contents {
						for _, op := range grp.Operations() {
							counter.With(prometheus.Labels{"kind": op.OperationKind()}).Inc()
						}
					}
				}
			}
		}
	}
}
