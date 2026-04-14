package main

import (
	"os"
)

type Config struct {
	BindAddr     string
	OpenAIAPIURL string
	OpenAIAPIKey string
	DefaultModel string
	LogFile      string
	LogEnabled   bool
	ConsoleLogOn bool
}

func LoadConfig() *Config {
	return &Config{
		BindAddr:     envOrDefault("BIND_ADDR", "0.0.0.0:8080"),
		OpenAIAPIURL: envOrDefault("OPENAI_API_URL", "https://api.openai.com"),
		OpenAIAPIKey: os.Getenv("OPENAI_API_KEY"),
		DefaultModel: envOrDefault("DEFAULT_MODEL", "gpt-4o"),
		LogFile:      envOrDefault("PROXY_LOG_FILE", "proxy.log"),
		LogEnabled:   envBoolOrDefault("PROXY_LOG_ENABLED", false),
		ConsoleLogOn: envBoolOrDefault("PROXY_CONSOLE_LOG", true),
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBoolOrDefault(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v == "1" || v == "true" || v == "yes"
}
