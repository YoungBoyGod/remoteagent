package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"luoyi2026/agent/internal/config"
	agentruntime "luoyi2026/agent/internal/runtime"
)

func main() {
	cfg := config.Load()
	agent := agentruntime.New(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := agent.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("agent exited with error: %v", err)
		os.Exit(1)
	}
	log.Printf("agent exited")
}
