package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	tz "github.com/ecadlabs/gotez/v2"
	client "github.com/ecadlabs/gotez/v2/clientv2"
	"github.com/ecadlabs/gotez/v2/clientv2/mempool"
	"github.com/ecadlabs/gotez/v2/clientv2/network"
	"github.com/ecadlabs/gotez/v2/clientv2/utils"
	"github.com/ecadlabs/gotez/v2/protocol/latest"
	"github.com/ecadlabs/gotez/v2/protocol/proto_016_PtMumbai"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

type PollerConfig struct {
	Client           *client.Client
	ChainID          *tz.ChainID
	Timeout          time.Duration
	Interval         time.Duration
	Reg              prometheus.Registerer
	NextProtocolFunc func() *tz.ProtocolHash
}

type Poller struct {
	cfg PollerConfig

	mtx    sync.RWMutex
	status utils.BootstrappedResponse

	cancel context.CancelFunc
	done   chan struct{}

	bsGauge   prometheus.Gauge
	connGauge *prometheus.GaugeVec
	opsGauge  *prometheus.GaugeVec
}

func (c *PollerConfig) New() *Poller {
	b := &Poller{
		cfg: *c,
		bsGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "tezos",
			Subsystem: "node",
			Name:      "bootstrapped",
			Help:      "Returns 1 if the node has synchronized its chain with a few peers.",
		}),
		connGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "tezos",
			Subsystem: "node",
			Name:      "connections",
			Help:      "Current number of connections to/from this node.",
		}, []string{"direction", "private"}),
		opsGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "tezos",
			Subsystem: "node",
			Name:      "mempool_operations",
			Help:      "The current number of mempool operations.",
		}, []string{"kind", "pool", "proto"}),
	}
	if c.Reg != nil {
		c.Reg.MustRegister(b.bsGauge)
		c.Reg.MustRegister(b.connGauge)
		c.Reg.MustRegister(b.opsGauge)
	}
	return b
}

func (p *Poller) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.done = make(chan struct{})
	go p.loop(ctx)
}

func (p *Poller) Stop(ctx context.Context) error {
	p.cancel()
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *Poller) Status() utils.BootstrappedResponse {
	p.mtx.RLock()
	defer p.mtx.RUnlock()
	return p.status
}

func (p *Poller) loop(ctx context.Context) {
	t := time.NewTicker(p.cfg.Interval)
	defer func() {
		t.Stop()
		close(p.done)
	}()

	pollers := []func(context.Context, chan<- error){
		p.pollBootstrapped,
		p.pollConnections,
		p.pollMempoolOperations,
	}
	errCh := make(chan error, len(pollers))

	for {
		for _, poller := range pollers {
			go poller(ctx, errCh)
		}
		done := false
		for range pollers {
			err := <-errCh
			if err != nil {
				if errors.Is(err, context.Canceled) {
					done = true
				} else {
					log.WithField("chain_id", p.cfg.ChainID).Warn(err)
				}
			}
		}
		if done {
			return
		}

		select {
		case <-t.C:
		case <-ctx.Done():
			return
		}
	}
}

func (p *Poller) pollBootstrapped(ctx context.Context, errCh chan<- error) {
	var err error
	defer func() { errCh <- err }()

	c, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()
	resp, err := utils.IsBootstrapped(c, p.cfg.Client, p.cfg.ChainID)
	if err != nil {
		return
	}
	p.mtx.Lock()
	p.status = *resp
	p.mtx.Unlock()
	v := 0.0
	if resp.SyncState == utils.SyncStateSynced && resp.Bootstrapped {
		v = 1
	}
	p.bsGauge.Set(v)
}

func (p *Poller) pollConnections(ctx context.Context, errCh chan<- error) {
	var err error
	defer func() { errCh <- err }()

	c, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()
	resp, err := network.Connections(c, p.cfg.Client)
	if err != nil {
		return
	}

	p.connGauge.Reset()
	for _, conn := range resp.Connections {
		var dir string
		if conn.Incoming {
			dir = "incoming"
		} else {
			dir = "outgoing"
		}
		p.connGauge.With(prometheus.Labels{"direction": dir, "private": fmt.Sprintf("%t", conn.Private)}).Inc()
	}
}

func updatePool(g *prometheus.GaugeVec, list []*proto_016_PtMumbai.OperationWithoutMetadata[latest.OperationContents]) {
	for _, grp := range list {
		for _, op := range grp.Operations() {
			g.With(prometheus.Labels{"kind": op.OperationKind()}).Inc()
		}
	}
}

func (p *Poller) pollMempoolOperations(ctx context.Context, errCh chan<- error) {
	var err error
	defer func() { errCh <- err }()

	c, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()
	resp, err := mempool.PendingOperations(c, p.cfg.Client, p.cfg.ChainID)
	if err != nil {
		return
	}

	p.opsGauge.Reset()
	gauge := p.opsGauge.MustCurryWith(prometheus.Labels{"proto": p.cfg.NextProtocolFunc().String()})

	g := gauge.MustCurryWith(prometheus.Labels{"pool": "validated"})
	for _, list := range resp.Validated {
		updatePool(g, list.Contents)
	}

	pools := [][]*mempool.PendingOperationsListWithError{
		resp.Refused,
		resp.Outdated,
		resp.BranchRefused,
		resp.BranchDelayed,
	}
	poolNames := []string{"refused", "outdated", "branch_refused", "branch_delayed"}
	for i, pool := range pools {
		g := gauge.MustCurryWith(prometheus.Labels{"pool": poolNames[i]})
		for _, list := range pool {
			updatePool(g, list.Contents)
		}
	}

	g = gauge.MustCurryWith(prometheus.Labels{"pool": "unprocessed"})
	for _, list := range resp.Unprocessed {
		updatePool(g, list.Contents)
	}
}
