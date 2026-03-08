package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Database struct {
		URL string `yaml:"url"`
	} `yaml:"database"`
	Scraper struct {
		Concurrency int `yaml:"concurrency"`
		TimeoutMs   int `yaml:"timeout_ms"`
	} `yaml:"scraper"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse yaml config: %w", err)
	}

	// Override with environment variables if present
	if envURL := os.Getenv("DB_URL"); envURL != "" {
		cfg.Database.URL = envURL
	}

	// Set defaults if missing
	if cfg.Scraper.Concurrency <= 0 {
		cfg.Scraper.Concurrency = 5
	}
	if cfg.Scraper.TimeoutMs <= 0 {
		cfg.Scraper.TimeoutMs = 30000
	}

	return &cfg, nil
}
