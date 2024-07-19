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
	Client  Client
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

func (s *HealthStatus) IsOk() bool {
	return s.BlockDelayOk && s.IsBootstrapped && s.IsSynced
}

func (h *HealthChecker) HealthStatus(ctx context.Context) (*HealthStatus, error) {
	status := HealthStatus{
		IsBootstrapped: true,
		IsSynced:       true,
	}
	if h.CheckBootstrapped || h.CheckSyncState {
		c, cancel := context.WithTimeout(ctx, h.Timeout)
		defer cancel()
		resp, err := h.Client.IsBootstrapped(c, h.ChainID)
		if err != nil {
			return nil, err
		}
		if h.CheckBootstrapped {
			status.IsBootstrapped = resp.Bootstrapped
		}
		if h.CheckSyncState {
			status.IsSynced = resp.SyncState == client.SyncStateSynced
		}
	}
	if h.CheckBlockDelay {
		status.BlockDelayOk = h.Monitor.Status()
	}

	if !status.IsOk() {
		log.WithFields(log.Fields{
			"chain_id":       h.ChainID,
			"bootstrapped":   status.IsBootstrapped,
			"synced":         status.IsSynced,
			"block_delay_ok": status.BlockDelayOk,
		}).Warn("Chain health is not ok")
	}
	return &status, nil
}
