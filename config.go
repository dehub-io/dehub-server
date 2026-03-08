package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the server configuration
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Upstream UpstreamConfig `yaml:"upstream"`
	Cache    CacheConfig    `yaml:"cache"`
}

type ServerConfig struct {
	Listen  string `yaml:"listen"`
	Storage string `yaml:"storage"`
}

type UpstreamConfig struct {
	URL     string `yaml:"url"`
	Enabled bool   `yaml:"enabled"`
}

type CacheConfig struct {
	Enabled bool   `yaml:"enabled"`
	TTL     string `yaml:"ttl"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	config := &Config{
		Server: ServerConfig{
			Listen:  ":8080",
			Storage: "./data",
		},
		Upstream: UpstreamConfig{
			Enabled: false,
		},
		Cache: CacheConfig{
			Enabled: true,
			TTL:     "168h",
		},
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return config, nil
}
