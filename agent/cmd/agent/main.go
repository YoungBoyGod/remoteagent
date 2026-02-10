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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		for sig := range sigCh {
			if sig == syscall.SIGHUP {
				agent.ReloadConfig()
				continue
			}
			log.Printf("shutdown signal received: %v", sig)
			cancel()
			return
		}
	}()

	if err := agent.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("agent exited with error: %v", err)
		os.Exit(1)
	}
	log.Printf("agent exited")
}
