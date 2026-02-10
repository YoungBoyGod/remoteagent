package server

import (
	"context"
	"net/http"

	"luoyi2026/server/internal/config"
	"luoyi2026/server/internal/handler"
	"luoyi2026/server/internal/state"
)

type Server struct {
	httpServer *http.Server
}

func New(cfg *config.Config, st *state.ServerState) *Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", handler.Health())
	mux.HandleFunc("/api/v1/agent/register", handler.Register(st, cfg))
	mux.HandleFunc("/api/v1/agent/heartbeat", handler.Heartbeat(st))
	mux.HandleFunc("/api/v1/agent/poll", handler.Poll(st, cfg))
	mux.HandleFunc("/api/v1/agent/task/status", handler.TaskStatus(st))
	mux.HandleFunc("/api/v1/agent/task/report", handler.TaskReport(st))
	mux.HandleFunc("/api/v1/debug/dispatch/task", handler.DebugDispatchTask(st, cfg))
	mux.HandleFunc("/api/v1/debug/dispatch/control", handler.DebugDispatchControl(st, cfg))
	mux.HandleFunc("/api/v1/debug/state", handler.DebugState(st, cfg))

	return &Server{
		httpServer: &http.Server{
			Addr:    cfg.Addr,
			Handler: mux,
		},
	}
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
