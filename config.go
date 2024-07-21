package main

import (
	"time"

	tz "github.com/ecadlabs/gotez/v2"
)

type Config struct {
	Listen                   string        `yaml:"listen"`
	URL                      string        `yaml:"url"`
	ChainID                  *tz.ChainID   `yaml:"chain_id"`
	Timeout                  time.Duration `yaml:"timeout"`
	Tolerance                time.Duration `yaml:"tolerance"`
	ReconnectDelay           time.Duration `yaml:"reconnect_delay"`
	UseTimestamps            bool          `yaml:"use_timestamps"`
	BootstrappedPollInterval time.Duration `yaml:"bootstrapped_poll_interval"`
	HealthUseBootstrapped    bool          `yaml:"health_use_bootstrapped"`
	HealthUseBlockDelay      bool          `yaml:"health_use_block_delay"`
}
