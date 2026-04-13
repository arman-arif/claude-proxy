package main

import (
	"os"
)

type Config struct {
	BindAddr      string
	OpenAIAPIURL  string
	OpenAIAPIKey  string
	DefaultModel  string
}

func LoadConfig() *Config {
	return &Config{
		BindAddr:     envOrDefault("BIND_ADDR", "0.0.0.0:8080"),
		OpenAIAPIURL: envOrDefault("OPENAI_API_URL", "https://api.openai.com"),
		OpenAIAPIKey: os.Getenv("OPENAI_API_KEY"),
		DefaultModel: envOrDefault("DEFAULT_MODEL", "gpt-4o"),
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
