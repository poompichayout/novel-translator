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
	Embedding struct {
		GeminiAPIKey string `yaml:"gemini_api_key"`
	} `yaml:"embedding"`
	LLM struct {
		AnthropicAPIKey string `yaml:"anthropic_api_key"`
	} `yaml:"llm"`
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

	if envURL := os.Getenv("DB_URL"); envURL != "" {
		cfg.Database.URL = envURL
	}
	if k := os.Getenv("GEMINI_API_KEY"); k != "" {
		cfg.Embedding.GeminiAPIKey = k
	}
	if k := os.Getenv("ANTHROPIC_API_KEY"); k != "" {
		cfg.LLM.AnthropicAPIKey = k
	}

	return &cfg, nil
}
