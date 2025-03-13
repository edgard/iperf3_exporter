// Copyright 2019 Edgard Castro
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package server provides the HTTP server for the iperf3 exporter.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/edgard/iperf3_exporter/internal/collector"
	"github.com/edgard/iperf3_exporter/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	versioncollector "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/exporter-toolkit/web"
)

// Server represents the HTTP server for the iperf3 exporter.
type Server struct {
	config *config.Config
	logger *slog.Logger
	server *http.Server
}

// New creates a new Server.
func New(cfg *config.Config) *Server {
	return &Server{
		config: cfg,
		logger: cfg.Logger,
	}
}

// setupServer configures the HTTP server with routes and middleware
func (s *Server) setupServer() {
	// Register Prometheus collectors
	prometheus.MustRegister(
		versioncollector.NewCollector("iperf3_exporter"),
		collectors.NewBuildInfoCollector(),
		collector.IperfDuration,
		collector.IperfErrors,
	)

	// Create router
	mux := http.NewServeMux()

	// Register handlers
	mux.Handle(s.config.MetricsPath, promhttp.Handler())
	mux.HandleFunc(s.config.ProbePath, s.probeHandler)
	mux.HandleFunc("/", s.indexHandler)
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/ready", s.readyHandler)

	// Register pprof handlers
	mux.HandleFunc("/debug/pprof/", http.DefaultServeMux.ServeHTTP)
	mux.HandleFunc("/debug/pprof/cmdline", http.DefaultServeMux.ServeHTTP)
	mux.HandleFunc("/debug/pprof/profile", http.DefaultServeMux.ServeHTTP)
	mux.HandleFunc("/debug/pprof/symbol", http.DefaultServeMux.ServeHTTP)
	mux.HandleFunc("/debug/pprof/trace", http.DefaultServeMux.ServeHTTP)
	mux.HandleFunc("/debug/pprof/heap", http.DefaultServeMux.ServeHTTP)

	// Create HTTP server with middleware
	s.server = &http.Server{
		Handler:      loggingMiddleware(mux, s.logger),
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Set up the server
	s.setupServer()

	// Start server using exporter-toolkit
	if err := web.ListenAndServe(s.server, s.config.WebConfig, s.logger); err != nil {
		return fmt.Errorf("error starting server: %w", err)
	}

	return nil
}

// Stop stops the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Stopping iperf3 exporter")
	return s.server.Shutdown(ctx)
}
