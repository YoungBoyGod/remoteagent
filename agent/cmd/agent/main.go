package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"luoyi2026/agent/internal/config"
)

type healthResp struct {
	Service   string `json:"service"`
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
}

func main() {
	cfg := config.Load()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(healthResp{
			Service:   "luoyi-agent",
			Status:    "ok",
			Timestamp: time.Now().Unix(),
		})
	})

	log.Printf("luoyi-agent local endpoint listening on %s", cfg.LocalAddr)
	if err := http.ListenAndServe(cfg.LocalAddr, mux); err != nil {
		log.Fatal(err)
	}
}
