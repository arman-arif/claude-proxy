package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	consoleLog := flag.Bool("log-console", true, "log requests to console")
	fileLog := flag.Bool("log-file", true, "log requests to file (PROXY_LOG_FILE)")
	flag.Parse()

	cfg := LoadConfig()

	// CLI flag overrides
	if !*consoleLog {
		cfg.ConsoleLogOn = false
	}
	if !*fileLog {
		cfg.LogEnabled = false
	}

	if cfg.OpenAIAPIKey == "" {
		log.Fatal("OPENAI_API_KEY required")
	}

	proxy := NewProxy(cfg)

	log.Printf("proxy listening on %s → %s/v1/chat/completions", cfg.BindAddr, cfg.OpenAIAPIURL)
	log.Printf("console log: %v | file log: %v (%s)", cfg.ConsoleLogOn, cfg.LogEnabled, cfg.LogFile)
	log.Fatal(http.ListenAndServe(cfg.BindAddr, proxy))
}
