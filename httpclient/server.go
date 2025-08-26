package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Server HTTP服务器封装
type Server struct {
	addr       string
	handler    http.Handler
	httpServer *http.Server
}

// NewServer 创建新的HTTP服务器
func NewServer(addr string, handler http.Handler) *Server {
	return &Server{
		addr:    addr,
		handler: handler,
	}
}

// Start 启动HTTP服务器
func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:         s.addr,
		Handler:      s.handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	return nil
}

// Shutdown 优雅关闭HTTP服务器
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}

	return s.httpServer.Shutdown(ctx)
}

// GetAddr 获取服务器地址
func (s *Server) GetAddr() string {
	return s.addr
}
