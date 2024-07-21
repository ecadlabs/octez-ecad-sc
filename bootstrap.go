package main

import (
	"context"
	"errors"
	"sync"
	"time"

	tz "github.com/ecadlabs/gotez/v2"
	"github.com/ecadlabs/gotez/v2/client"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

type BootstrapPollerConfig struct {
	Client   Client
	ChainID  *tz.ChainID
	Timeout  time.Duration
	Interval time.Duration
	Reg      prometheus.Registerer
}

type BootstrapPoller struct {
	cfg BootstrapPollerConfig

	mtx    sync.RWMutex
	status client.BootstrappedResponse

	cancel context.CancelFunc
	done   chan struct{}
	metric prometheus.Gauge
}

func (c *BootstrapPollerConfig) New() *BootstrapPoller {
	b := &BootstrapPoller{
		cfg: *c,
		metric: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "tezos",
			Subsystem: "node",
			Name:      "bootstrapped",
			Help:      "Returns 1 if the node has synchronized its chain with a few peers.",
		}),
	}
	if c.Reg != nil {
		c.Reg.MustRegister(b.metric)
	}
	return b
}

func (b *BootstrapPoller) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	b.done = make(chan struct{})
	go b.loop(ctx)
}

func (b *BootstrapPoller) Stop(ctx context.Context) error {
	b.cancel()
	select {
	case <-b.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *BootstrapPoller) Status() client.BootstrappedResponse {
	b.mtx.RLock()
	defer b.mtx.RUnlock()
	return b.status
}

func (b *BootstrapPoller) loop(ctx context.Context) {
	t := time.NewTicker(b.cfg.Interval)
	defer func() {
		t.Stop()
		close(b.done)
	}()
	for {
		c, cancel := context.WithTimeout(ctx, b.cfg.Timeout)
		resp, err := b.cfg.Client.IsBootstrapped(c, b.cfg.ChainID)
		cancel()
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			log.WithField("chain_id", b.cfg.ChainID).Warn(err)
		} else {
			b.mtx.Lock()
			b.status = *resp
			b.mtx.Unlock()
			v := 0.0
			if resp.SyncState == client.SyncStateSynced && resp.Bootstrapped {
				v = 1
			}
			b.metric.Set(v)
		}

		select {
		case <-t.C:
		case <-ctx.Done():
			return
		}
	}
}
