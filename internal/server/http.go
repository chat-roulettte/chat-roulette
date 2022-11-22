package server

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
)

// Start runs the HTTP server
func (s *Server) Start(ctx context.Context, ch chan error) {
	logger := hclog.FromContext(ctx)

	logger.Info("starting the HTTP server", "addr", s.config.GetAddr())
	ch <- s.httpServer.ListenAndServe()
}

// Stop gracefully stops the HTTP server
func (s *Server) Stop(ctx context.Context, wg *sync.WaitGroup, stop chan os.Signal, ch chan error) {
	defer wg.Done()

	logger := hclog.FromContext(ctx)

	select {
	case <-stop:
		logger.Info("received signal, gracefully shutting down HTTP server")

	case err := <-ch:
		logger.Error("failed to start the HTTP server", "error", err)
	}

	logger.Info("waiting up to 10 seconds for in-flight HTTP requests to complete")

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		logger.Error("failed to shutdown HTTP server", "error", err)
	}
}
