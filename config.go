package main

import (
	"time"

	tz "github.com/ecadlabs/gotez/v2"
)

type NodeConfig struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

type Config struct {
	Listen                string        `yaml:"listen"`
	Nodes                 []NodeConfig  `yaml:"nodes"`
	URL                   string        `yaml:"url"` // legacy single-node field
	ChainID               *tz.ChainID   `yaml:"chain_id"`
	Timeout               time.Duration `yaml:"timeout"`
	Tolerance             time.Duration `yaml:"tolerance"`
	ReconnectDelay        time.Duration `yaml:"reconnect_delay"`
	UseTimestamps         bool          `yaml:"use_timestamps"`
	PollInterval          time.Duration `yaml:"poll_interval"`
	HealthUseBootstrapped bool          `yaml:"health_use_bootstrapped"`
	HealthUseBlockDelay   bool          `yaml:"health_use_block_delay"`
}
