package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"luoyi2026/agent/internal/config"
	"luoyi2026/agent/internal/logging"
	agentruntime "luoyi2026/agent/internal/runtime"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	log.Printf("luoyi-agent %s (commit=%s, built=%s)", version, commit, buildTime)
	cfg, err := config.Load()
	if err != nil {
		log.Printf("load config failed: %v", err)
		os.Exit(1)
	}
	cleanupLogger, err := logging.Setup(cfg)
	if err != nil {
		log.Printf("init logger failed: %v", err)
		os.Exit(1)
	}
	defer cleanupLogger()

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
