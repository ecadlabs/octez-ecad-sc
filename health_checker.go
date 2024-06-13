package main

import (
	"context"
	"time"

	tz "github.com/ecadlabs/gotez/v2"
	"github.com/ecadlabs/gotez/v2/client"
	log "github.com/sirupsen/logrus"
)

type HealthChecker struct {
	Monitor *HeadMonitor
	Client  *client.Client
	ChainID *tz.ChainID
	Timeout time.Duration

	CheckBlockDelay   bool
	CheckBootstrapped bool
	CheckSyncState    bool
}

type HealthStatus struct {
	IsBootstrapped bool `json:"bootstrapped"`
	IsSynced       bool `json:"synced"`
	BlockDelayOk   bool `json:"block_delay_ok"`
}

func (h *HealthChecker) HealthStatus(ctx context.Context) (*HealthStatus, bool, error) {
	var status HealthStatus
	ok := true
	if h.CheckBootstrapped || h.CheckSyncState {
		c, cancel := context.WithTimeout(ctx, h.Timeout)
		defer cancel()
		resp, err := h.Client.IsBootstrapped(c, h.ChainID)
		if err != nil {
			return nil, false, err
		}
		if h.CheckBootstrapped {
			status.IsBootstrapped = resp.Bootstrapped
			ok = ok && status.IsBootstrapped
		}
		if h.CheckSyncState {
			status.IsSynced = resp.SyncState == client.SyncStateSynced
			ok = ok && status.IsSynced
		}
	}
	if h.CheckBlockDelay {
		status.BlockDelayOk = h.Monitor.Status()
		ok = ok && status.BlockDelayOk
	}

	if !ok {
		log.WithFields(log.Fields{
			"chain_id":       h.ChainID,
			"bootstrapped":   status.IsBootstrapped,
			"synced":         status.IsSynced,
			"block_delay_ok": status.BlockDelayOk,
		}).Warn("Chain health is not ok")
	}
	return &status, ok, nil
}
