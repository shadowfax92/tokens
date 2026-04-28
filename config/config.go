package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DefaultDays     int `yaml:"default_days"`
	CacheTTLMinutes int `yaml:"cache_ttl_minutes"`
}

func defaults() *Config {
	return &Config{
		DefaultDays:     14,
		CacheTTLMinutes: 5,
	}
}

func Path() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "tokens", "config.yaml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "tokens", "config.yaml")
}

func Load() (*Config, error) {
	cfg := defaults()
	data, err := os.ReadFile(Path())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if cfg.DefaultDays <= 0 {
		cfg.DefaultDays = 14
	}
	if cfg.CacheTTLMinutes <= 0 {
		cfg.CacheTTLMinutes = 5
	}
	return cfg, nil
}

func EnsureExists() error {
	p := Path()
	if _, err := os.Stat(p); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(defaults())
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}
