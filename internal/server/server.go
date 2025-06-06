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
	"strconv"
	"time"

	"github.com/edgard/iperf3_exporter/internal/collector"
	"github.com/edgard/iperf3_exporter/internal/config"
	"github.com/edgard/iperf3_exporter/internal/iperf"
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

// Start starts the HTTP server.
func (s *Server) Start() error {
	// Register version and process collectors
	prometheus.MustRegister(versioncollector.NewCollector("iperf3_exporter"))
	prometheus.MustRegister(collectors.NewBuildInfoCollector())
	prometheus.MustRegister(collector.IperfDuration)
	prometheus.MustRegister(collector.IperfErrors)

	// Create router
	mux := http.NewServeMux()

	// Add middleware
	var handler http.Handler = mux
	handler = s.withLogging(handler)

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

	// Create HTTP server
	s.server = &http.Server{
		Handler:      handler,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

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

// probeHandler handles requests to the /probe endpoint.
func (s *Server) probeHandler(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("target")
	if target == "" {
		http.Error(w, "'target' parameter must be specified", http.StatusBadRequest)
		collector.IperfErrors.Inc()

		return
	}

	var targetPort int

	port := r.URL.Query().Get("port")
	if port != "" {
		var err error

		targetPort, err = strconv.Atoi(port)
		if err != nil {
			http.Error(w, fmt.Sprintf("'port' parameter must be an integer: %s", err), http.StatusBadRequest)
			collector.IperfErrors.Inc()

			return
		}
	}

	if targetPort == 0 {
		targetPort = 5201
	}

	var reverseMode bool

	reverseParam := r.URL.Query().Get("reverse_mode")
	if reverseParam != "" {
		var err error

		reverseMode, err = strconv.ParseBool(reverseParam)
		if err != nil {
			http.Error(w, fmt.Sprintf("'reverse_mode' parameter must be true or false (boolean): %s", err), http.StatusBadRequest)
			collector.IperfErrors.Inc()

			return
		}
	}

	var udpMode bool

	udpModeParam := r.URL.Query().Get("udp_mode")
	if udpModeParam != "" {
		var err error

		udpMode, err = strconv.ParseBool(udpModeParam)
		if err != nil {
			http.Error(w, fmt.Sprintf("'udp_mode' parameter must be true or false (boolean): %s", err), http.StatusBadRequest)
			collector.IperfErrors.Inc()

			return
		}
	}

	bitrate := r.URL.Query().Get("bitrate")
	if bitrate != "" && !iperf.ValidateBitrate(bitrate) {
		http.Error(w, "bitrate must provided as #[KMG][/#], target bitrate in bits/sec (0 for unlimited), (default 1 Mbit/sec for UDP, unlimited for TCP) (optional slash and packet count for burst mode)", http.StatusBadRequest)
		collector.IperfErrors.Inc()

		return
	}

	// Note: In UDP mode, iperf3 requires a bitrate (defaults to 1Mbps if not specified)
	// Add a log message for clarity if udpMode is enabled but no bitrate specified
	if udpMode && bitrate == "" {
		s.logger.Info("UDP mode is enabled but no bitrate specified - iperf3 will use the default of 1Mbps")
	}

	var runPeriod time.Duration

	period := r.URL.Query().Get("period")
	if period != "" {
		var err error

		runPeriod, err = time.ParseDuration(period)
		if err != nil {
			http.Error(w, fmt.Sprintf("'period' parameter must be a duration: %s", err), http.StatusBadRequest)
			collector.IperfErrors.Inc()

			return
		}
	}

	if runPeriod.Seconds() == 0 {
		runPeriod = time.Second * 5
	}

	// If a timeout is configured via the Prometheus header, add it to the request.
	var timeoutSeconds float64

	if v := r.Header.Get("X-Prometheus-Scrape-Timeout-Seconds"); v != "" {
		var err error

		timeoutSeconds, err = strconv.ParseFloat(v, 64)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse timeout from Prometheus header: %s", err), http.StatusInternalServerError)
			collector.IperfErrors.Inc()

			return
		}
	}

	if timeoutSeconds == 0 {
		if s.config.Timeout.Seconds() > 0 {
			timeoutSeconds = s.config.Timeout.Seconds()
		} else {
			timeoutSeconds = 30
		}
	}

	// Ensure run period is less than timeout to avoid premature termination
	if runPeriod.Seconds() >= timeoutSeconds {
		runPeriod = time.Duration(timeoutSeconds*0.9) * time.Second
	}

	runTimeout := time.Duration(timeoutSeconds * float64(time.Second))

	start := time.Now()
	registry := prometheus.NewRegistry()

	// Create collector with probe configuration
	probeConfig := collector.ProbeConfig{
		Target:      target,
		Port:        targetPort,
		Period:      runPeriod,
		Timeout:     runTimeout,
		ReverseMode: reverseMode,
		UDPMode:     udpMode,
		Bitrate:     bitrate,
	}

	c := collector.NewCollector(probeConfig, s.logger)
	registry.MustRegister(c)

	// Delegate http serving to Prometheus client library, which will call collector.Collect.
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)

	duration := time.Since(start).Seconds()
	collector.IperfDuration.Observe(duration)
}

// indexHandler handles requests to the / endpoint using the exporter-toolkit landing page.
func (s *Server) indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)

		return
	}

	// Get landing page configuration from config
	landingConfig := s.config.GetLandingConfig()

	// Create and serve the landing page
	landingPage, err := web.NewLandingPage(landingConfig)
	if err != nil {
		s.logger.Warn("Failed to create landing page", "err", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)

		return
	}

	landingPage.ServeHTTP(w, r)
}

// healthHandler handles requests to the /health endpoint.
func (s *Server) healthHandler(w http.ResponseWriter, _ *http.Request) {
	// Check if iperf3 exists
	if err := iperf.CheckIperf3Exists(); err != nil {
		s.logger.Error("iperf3 command not found", "err", err)
		http.Error(w, "iperf3 command not found", http.StatusServiceUnavailable)

		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "OK")
}

// readyHandler handles requests to the /ready endpoint.
func (s *Server) readyHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Ready")
}

// withLogging adds logging middleware to the HTTP handler.
func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a custom response writer to capture the status code
		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(rw, r)

		duration := time.Since(start)

		s.logger.Debug("HTTP request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration", duration.String(),
			"remote_addr", r.RemoteAddr,
		)
	})
}

// responseWriter is a custom response writer that captures the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code.
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
