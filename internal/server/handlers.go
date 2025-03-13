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

package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/edgard/iperf3_exporter/internal/collector"
	"github.com/edgard/iperf3_exporter/internal/iperf"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/exporter-toolkit/web"
)

// probeHandler handles requests to the /probe endpoint.
func (s *Server) probeHandler(w http.ResponseWriter, r *http.Request) {
	probeReq, err := ParseProbeRequest(r)
	if err != nil {
		s.logger.Error("Invalid probe request", "err", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		collector.IperfErrors.Inc()
		return
	}

	start := time.Now()
	registry := prometheus.NewRegistry()

	// Create collector with probe configuration
	c := collector.NewCollector(collector.ProbeConfig{
		Target:      probeReq.Target,
		Port:        probeReq.Port,
		Period:      probeReq.Period,
		Timeout:     probeReq.Timeout,
		ReverseMode: probeReq.ReverseMode,
		Bitrate:     probeReq.Bitrate,
	}, s.logger)
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
