package main

import (
	"time"

	tz "github.com/ecadlabs/gotez/v2"
)

type Config struct {
	Listen            string        `yaml:"listen"`
	URL               string        `yaml:"url"`
	ChainID           *tz.ChainID   `yaml:"chain_id"`
	Timeout           time.Duration `yaml:"timeout"`
	Tolerance         time.Duration `yaml:"tolerance"`
	ReconnectDelay    time.Duration `yaml:"reconnect_delay"`
	UseTimestamps     bool          `yaml:"use_timestamps"`
	CheckBlockDelay   bool          `yaml:"check_block_delay"`
	CheckBootstrapped bool          `yaml:"check_bootstrapped"`
	CheckSyncState    bool          `yaml:"check_sync_state"`
}
