package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	MountPoints []MountPoint `yaml:"mount_points"`
	Port        int          `yaml:"port"`
	Address     string       `yaml:"address"`
}

type MountPoint struct {
	Path      string `yaml:"path"`
	Label     string `yaml:"label"`
	MaxSizeMB int    `yaml:"max_size_mb"`
}

func LoadConfig(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return cfg, err
	}
	if cfg.Port == 0 {
		cfg.Port = 8080
	}
	if cfg.Address == "" {
		cfg.Address = "0.0.0.0"
	}
	return cfg, nil
}
