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
	// Parse command line flags
	cfg := config.ParseFlags()

	// Log version and build information
	cfg.Logger.Info("Starting iperf3 exporter")

	// Check if iperf3 exists
	if err := iperf.CheckIperf3Exists(); err != nil {
		cfg.Logger.Error("iperf3 command not found, please install iperf3", "err", err)
		os.Exit(1)
	}

	// Create and start HTTP server
	srv := server.New(cfg)

	// Setup graceful shutdown
	done := make(chan struct{})
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		sig := <-sigChan
		cfg.Logger.Info("Received signal, shutting down gracefully", "signal", sig.String())

		// Create a context with timeout for shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Stop(ctx); err != nil {
			cfg.Logger.Error("HTTP server shutdown error", "err", err)
		}

		close(done)
	}()

	// Start the server
	if err := srv.Start(); err != nil {
		cfg.Logger.Error("HTTP server error", "err", err)
		os.Exit(1)
	}

	<-done
	cfg.Logger.Info("Server stopped gracefully")
}
