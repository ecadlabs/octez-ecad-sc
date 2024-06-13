package main

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	tz "github.com/ecadlabs/gotez/v2"
	"github.com/ecadlabs/gotez/v2/client"
	"github.com/ecadlabs/gotez/v2/protocol/core"
	log "github.com/sirupsen/logrus"
)

type HeadMonitor struct {
	Client         *client.Client
	ChainID        *tz.ChainID
	Timeout        time.Duration
	Tolerance      time.Duration
	ReconnectDelay time.Duration
	UseTimestamps  bool

	status atomic.Bool
	cancel context.CancelFunc
	done   chan struct{}
}

func (h *HeadMonitor) Status() bool {
	return h.status.Load()
}

func (h *HeadMonitor) context(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, h.Timeout)
}

func (h *HeadMonitor) getMinBlockDelay(c context.Context, block string, protocol *tz.ProtocolHash) (time.Duration, error) {
	ctx, cancel := h.context(c)
	defer cancel()
	consts, err := h.Client.Constants(ctx, &client.ContextRequest{
		Chain:    h.ChainID.String(),
		Block:    block,
		Protocol: protocol,
	})
	if err != nil {
		return 0, err
	}
	delay := time.Duration(consts.GetMinimalBlockDelay()) * time.Second
	log.Debugf("%s delay = %v", block, delay)
	return delay, nil
}

func (h *HeadMonitor) getShellHeader(c context.Context, block *tz.BlockHash) (*core.ShellHeader, error) {
	ctx, cancel := h.context(c)
	defer cancel()
	return h.Client.BlockShellHeader(ctx, &client.SimpleRequest{
		Chain: h.ChainID.String(),
		Block: block.String(),
	})
}

func (h *HeadMonitor) getBlockInfo(c context.Context, block string) (*client.BasicBlockInfo, error) {
	ctx, cancel := h.context(c)
	defer cancel()
	return h.Client.BasicBlockInfo(ctx, h.ChainID.String(), block)
}

func (h *HeadMonitor) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	h.done = make(chan struct{})
	go h.serve(ctx)
}

func (h *HeadMonitor) Stop(ctx context.Context) error {
	h.cancel()
	select {
	case <-h.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *HeadMonitor) serve(ctx context.Context) {
	defer close(h.done)
	var err error
	for {
		h.status.Store(false)
		if err != nil {
			log.Error(err)
			t := time.After(h.ReconnectDelay)
			select {
			case <-t:
			case <-ctx.Done():
				return
			}
		}

		var bi *client.BasicBlockInfo
		bi, err = h.getBlockInfo(ctx, "head")
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			continue
		}
		var sh *core.ShellHeader
		sh, err = h.getShellHeader(ctx, bi.Hash)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			continue
		}
		var timestamp time.Time
		if h.UseTimestamps {
			timestamp = sh.Timestamp.Time()
		} else {
			timestamp = time.Now()
		}

		protoNum := sh.Proto
		var minBlockDelay time.Duration
		minBlockDelay, err = h.getMinBlockDelay(ctx, bi.Hash.String(), bi.Protocol)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			continue
		}
		var (
			stream <-chan *client.Head
			errCh  <-chan error
		)
		stream, errCh, err = h.Client.Heads(ctx, &client.HeadsRequest{Chain: h.ChainID.String()})
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

			case head := <-stream:
				var t time.Time
				if h.UseTimestamps {
					t = head.Timestamp.Time()
				} else {
					t = time.Now()
				}
				status := t.Before(timestamp.Add(minBlockDelay + h.Tolerance))
				log.Debugf("%v: %t", t, status)
				h.status.Store(status)
				timestamp = t
				if head.Proto == protoNum {
					break
				}

				// update constant
				var proto *core.BlockProtocols
				proto, err = h.Client.BlockProtocols(ctx, &client.SimpleRequest{
					Chain: h.ChainID.String(),
					Block: head.Hash.String(),
				})
				if err != nil {
					if errors.Is(err, context.Canceled) {
						return
					}
					break Recv
				}
				minBlockDelay, err = h.getMinBlockDelay(ctx, head.Hash.String(), proto.Protocol)
				if err != nil {
					if errors.Is(err, context.Canceled) {
						return
					}
					break Recv
				}
				protoNum = head.Proto
			}
		}
	}
}
