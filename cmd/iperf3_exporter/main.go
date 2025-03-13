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

// Package main provides the entry point for the iperf3_exporter.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/edgard/iperf3_exporter/internal/config"
	"github.com/edgard/iperf3_exporter/internal/iperf"
	"github.com/edgard/iperf3_exporter/internal/server"
)

func main() {
	cfg := config.ParseFlags()
	cfg.Logger.Info("Starting iperf3 exporter")

	if err := iperf.CheckIperf3Exists(); err != nil {
		cfg.Logger.Error("iperf3 command not found, please install iperf3", "err", err)
		os.Exit(1)
	}

	srv := server.New(cfg)

	// Setup graceful shutdown using signal.NotifyContext
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start server in goroutine
	go func() {
		if err := srv.Start(); err != nil {
			cfg.Logger.Error("HTTP server error", "err", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	<-ctx.Done()
	cfg.Logger.Info("Shutting down gracefully")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Stop(shutdownCtx); err != nil {
		cfg.Logger.Error("Shutdown error", "err", err)
		os.Exit(1)
	}

	cfg.Logger.Info("Server stopped gracefully")
}
