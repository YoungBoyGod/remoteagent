package app

import (
	"context"
	"net/http"

	"luoyi2026/server/internal/config"
	"luoyi2026/server/internal/router"
	"luoyi2026/server/internal/service"
)

type Server struct {
	httpServer *http.Server
}

func New(cfg *config.Config, svc *service.Service) *Server {
	engine := router.Setup(cfg, svc)
	return &Server{
		httpServer: &http.Server{
			Addr:    cfg.Addr,
			Handler: engine,
		},
	}
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
