// @title Luoyi Remote Agent Server API
// @version 1.0
// @description 远程 Agent 管理服务，提供注册、心跳、任务下发与上报等接口。

// @host localhost:40001
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

// @securityDefinitions.apikey AdminAuth
// @in header
// @name X-Register-Token

package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	_ "luoyi2026/server/api"
	"luoyi2026/server/internal/app"
	"luoyi2026/server/internal/config"
	"luoyi2026/server/internal/service"
)

func main() {
	cfg := config.Load()

	db, err := sql.Open("postgres", cfg.PostgresDSN())
	if err != nil {
		log.Fatalf("open postgres failed: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.DBConnectTimeoutS)*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("ping postgres failed: %v", err)
	}
	log.Printf("postgres connected: %s:%d/%s", cfg.DBHost, cfg.DBPort, cfg.DBName)

	svc := service.New(db)
	srv := app.New(&cfg, svc)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		log.Printf("luoyi-server listening on %s", cfg.Addr)
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	for sig := range sigCh {
		if sig == syscall.SIGHUP {
			cfg.ReloadFrom()
			log.Printf("config reloaded")
			continue
		}
		log.Printf("shutdown signal received: %v", sig)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
		shutdownCancel()
		return
	}
}
