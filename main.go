package main

import (
	"log"
	"net/http"
)

func main() {
	cfg := LoadConfig()

	if cfg.OpenAIAPIKey == "" {
		log.Fatal("OPENAI_API_KEY required")
	}

	proxy := NewProxy(cfg)

	log.Printf("proxy listening on %s → %s/v1/chat/completions", cfg.BindAddr, cfg.OpenAIAPIURL)
	log.Fatal(http.ListenAndServe(cfg.BindAddr, proxy))
}
