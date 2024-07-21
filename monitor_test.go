package main

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ecadlabs/gotez/v2"
	"github.com/ecadlabs/gotez/v2/client"
	"github.com/ecadlabs/gotez/v2/protocol/core"
	"github.com/ecadlabs/gotez/v2/protocol/proto_019_PtParisB"
	"github.com/stretchr/testify/require"
)

type Call struct {
	Func string
	Args []any
	Ret  []any
}

type blockContext struct {
	head      client.Head
	proto     gotez.ProtocolHash
	constants proto_019_PtParisB.Constants
}

type clientMockup struct {
	mtx      sync.Mutex
	calls    []*Call
	head     int64
	contexts []*blockContext
	done     chan struct{}
}

func (m *clientMockup) call(args, ret []any) {
	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		return
	}
	m.callFunc(runtime.FuncForPC(pc).Name(), args, ret)
}

func (m *clientMockup) callFunc(fn string, args, ret []any) {
	c := &Call{
		Args: args,
		Ret:  ret,
		Func: fn,
	}
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.calls = append(m.calls, c)
}

func (c *clientMockup) ctx(block string) (*blockContext, error) {
	if block == "head" {
		return c.contexts[atomic.LoadInt64(&c.head)], nil
	}
	for _, bc := range c.contexts {
		if bc.head.Hash.String() == block {
			return bc, nil
		}
	}
	return nil, errors.New("block not found")
}

func (c *clientMockup) Constants(ctx context.Context, r *client.ContextRequest) (client.Constants, error) {
	bc, err := c.ctx(r.Block)
	if err != nil {
		c.call([]any{r}, []any{client.Constants(nil), err})
		return nil, err
	}
	c.call([]any{r}, []any{&bc.constants, error(nil)})
	return &bc.constants, nil
}

func (c *clientMockup) BlockShellHeader(ctx context.Context, r *client.SimpleRequest) (*client.BlockShellHeader, error) {
	bc, err := c.ctx(r.Block)
	if err != nil {
		c.call([]any{r}, []any{(*client.BlockShellHeader)(nil), err})
		return nil, err
	}
	c.call([]any{r}, []any{&bc.head.ShellHeader, error(nil)})
	return &bc.head.ShellHeader, nil
}

func (c *clientMockup) BlockProtocols(ctx context.Context, r *client.SimpleRequest) (*client.BlockProtocols, error) {
	bc, err := c.ctx(r.Block)
	if err != nil {
		c.call([]any{r}, []any{(*client.BlockProtocols)(nil), err})
		return nil, err
	}
	proto := &client.BlockProtocols{Protocol: &bc.proto}
	c.call([]any{r}, []any{proto, error(nil)})
	return proto, nil
}

func (c *clientMockup) BasicBlockInfo(ctx context.Context, chain string, block string) (*client.BasicBlockInfo, error) {
	bc, err := c.ctx(block)
	if err != nil {
		c.call([]any{chain, block}, []any{(*client.BasicBlockInfo)(nil), err})
		return nil, err
	}
	bi := &client.BasicBlockInfo{
		Hash:     bc.head.Hash,
		Protocol: &bc.proto,
	}
	c.call([]any{chain, block}, []any{bi, error(nil)})
	return bi, nil
}

func (c *clientMockup) IsBootstrapped(ctx context.Context, r *client.ChainID) (*client.BootstrappedResponse, error) {
	status := &client.BootstrappedResponse{Bootstrapped: true, SyncState: client.SyncStateSynced}
	c.call([]any{r}, []any{status, error(nil)})
	return status, nil
}

func (c *clientMockup) Heads(ctx context.Context, r *client.HeadsRequest) (<-chan *client.Head, <-chan error, error) {
	heads := make(chan *client.Head)
	err := make(chan error)
	go func() {
		for {
			head := atomic.AddInt64(&c.head, 1)
			if int(head) == len(c.contexts) {
				break
			}
			bc := c.contexts[head]
			select {
			case heads <- &bc.head:
				c.callFunc("", nil, []any{&bc.head})
			case <-ctx.Done():
				err <- ctx.Err()
				return
			}
		}
		close(c.done)
		// just wait for a context to close
		<-ctx.Done()
		err <- ctx.Err()
	}()
	c.call([]any{r}, []any{error(nil)})
	return heads, err, nil
}

func TestProtocolUpgrade1(t *testing.T) {
	cl := clientMockup{
		done: make(chan struct{}),
		contexts: []*blockContext{
			{
				head: client.Head{
					Hash: &gotez.BlockHash{1},
					ShellHeader: core.ShellHeader{
						Proto:     0,
						Timestamp: 0,
					},
				},
				proto:     gotez.ProtocolHash{1},
				constants: proto_019_PtParisB.Constants{MinimalBlockDelay: 1},
			},
			{
				head: client.Head{
					Hash: &gotez.BlockHash{2},
					ShellHeader: core.ShellHeader{
						Proto:     1,
						Timestamp: 1,
					},
				},
				proto:     gotez.ProtocolHash{2},
				constants: proto_019_PtParisB.Constants{MinimalBlockDelay: 1},
			},
		},
	}

	expect := []*Call{
		{
			Func: "github.com/ecadlabs/octez-ecad-sc.(*clientMockup).BasicBlockInfo",
			Args: []any{"NetXH12Aer3be93", "head"},
			Ret: []any{&client.BasicBlockInfo{
				Hash:     cl.contexts[0].head.Hash,
				Protocol: &cl.contexts[0].proto,
			}, error(nil)},
		},
		{
			Func: "github.com/ecadlabs/octez-ecad-sc.(*clientMockup).BlockShellHeader",
			Args: []any{&client.SimpleRequest{
				Chain: "NetXH12Aer3be93",
				Block: "BKiisx71SeX91a4DF6vd4ykBkDTdSVpkH44SvxUc9U8ytodDvfn",
			}},
			Ret: []any{&cl.contexts[0].head.ShellHeader, error(nil)},
		},
		{
			Func: "github.com/ecadlabs/octez-ecad-sc.(*clientMockup).Constants",
			Args: []any{&client.ContextRequest{
				Chain:    "NetXH12Aer3be93",
				Block:    "BKiisx71SeX91a4DF6vd4ykBkDTdSVpkH44SvxUc9U8ytodDvfn",
				Protocol: &cl.contexts[0].proto,
			}},
			Ret: []any{&cl.contexts[0].constants, error(nil)},
		},
		{
			Func: "github.com/ecadlabs/octez-ecad-sc.(*clientMockup).Heads",
			Args: []any{&client.HeadsRequest{
				Chain: "NetXH12Aer3be93",
			}},
			Ret: []any{error(nil)},
		},
		{
			Ret: []any{&cl.contexts[1].head},
		},
		{
			Func: "github.com/ecadlabs/octez-ecad-sc.(*clientMockup).BlockProtocols",
			Args: []any{&client.SimpleRequest{
				Chain: "NetXH12Aer3be93",
				Block: "BKjARUyBRFjXVU8CGfgVNBprSJF5f76oCFJjin6787DWo5AnU9J",
			}},
			Ret: []any{&client.BlockProtocols{Protocol: &cl.contexts[1].proto}, error(nil)},
		},
		{
			Func: "github.com/ecadlabs/octez-ecad-sc.(*clientMockup).Constants",
			Args: []any{&client.ContextRequest{
				Chain:    "NetXH12Aer3be93",
				Block:    "BKjARUyBRFjXVU8CGfgVNBprSJF5f76oCFJjin6787DWo5AnU9J",
				Protocol: &cl.contexts[1].proto,
			}},
			Ret: []any{&cl.contexts[1].constants, error(nil)},
		},
	}

	mon := (&HeadMonitorConfig{
		Client:        &cl,
		ChainID:       &gotez.ChainID{0},
		UseTimestamps: true,
	}).New()

	mon.Start()
	<-cl.done
	mon.Stop(context.Background())

	require.Equal(t, expect, cl.calls)
}
