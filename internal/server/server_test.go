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
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/edgard/iperf3_exporter/internal/config"
	"github.com/prometheus/exporter-toolkit/web"
)

// setupTest initializes common test resources.
func setupTest(t *testing.T) (*config.Config, *slog.Logger) {
	t.Helper()

	// Create a test logger that writes to stderr
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	// Create a test configuration
	cfg := config.NewConfig()
	cfg.Logger = logger

	// Use a test web config
	cfg.WebConfig = &web.FlagConfig{}

	return cfg, logger
}

// TestNew tests that New creates a new Server.
func TestNew(t *testing.T) {
	cfg, _ := setupTest(t)

	srv := New(cfg)
	if srv == nil {
		t.Fatal("New() returned nil")
	}

	if srv.config != cfg {
		t.Errorf("New() config = %v, want %v", srv.config, cfg)
	}

	if srv.logger != cfg.Logger {
		t.Errorf("New() logger = %v, want %v", srv.logger, cfg.Logger)
	}

	if srv.server != nil {
		t.Errorf("New() server = %v, want nil", srv.server)
	}
}

// TestIndexHandler tests the index handler.
func TestIndexHandler(t *testing.T) {
	cfg, _ := setupTest(t)
	srv := New(cfg)

	// Create a test request
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	// Call the handler
	srv.indexHandler(w, req)

	// Check the response
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("indexHandler() status = %v, want %v", resp.StatusCode, http.StatusOK)
	}

	// Check that the response contains the expected content
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Check that the response contains the expected content
	expectedContent := []string{
		"iPerf3 Exporter",
		"Metrics",
		"GitHub Repository",
		"Quick Start",
		"Probe Parameters",
	}

	for _, expected := range expectedContent {
		if !strings.Contains(string(body), expected) {
			t.Errorf("indexHandler() response does not contain %q", expected)
		}
	}
}

// TestIndexHandlerNotFound tests the index handler with a non-root path.
func TestIndexHandlerNotFound(t *testing.T) {
	cfg, _ := setupTest(t)
	srv := New(cfg)

	// Create a test request with a non-root path
	req := httptest.NewRequest(http.MethodGet, "/not-found", nil)
	w := httptest.NewRecorder()

	// Call the handler
	srv.indexHandler(w, req)

	// Check the response
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("indexHandler() status = %v, want %v", resp.StatusCode, http.StatusNotFound)
	}
}

// TestHealthHandler tests the health handler.
func TestHealthHandler(t *testing.T) {
	cfg, _ := setupTest(t)
	srv := New(cfg)

	// Create a test request
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	// Call the handler
	srv.healthHandler(w, req)

	// Check the response
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("healthHandler() status = %v, want %v", resp.StatusCode, http.StatusOK)
	}

	// Check that the response contains the expected content
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if string(body) != "OK\n" {
		t.Errorf("healthHandler() response = %q, want %q", string(body), "OK\n")
	}
}

// TestReadyHandler tests the ready handler.
func TestReadyHandler(t *testing.T) {
	cfg, _ := setupTest(t)
	srv := New(cfg)

	// Create a test request
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	// Call the handler
	srv.readyHandler(w, req)

	// Check the response
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("readyHandler() status = %v, want %v", resp.StatusCode, http.StatusOK)
	}

	// Check that the response contains the expected content
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if string(body) != "Ready\n" {
		t.Errorf("readyHandler() response = %q, want %q", string(body), "Ready\n")
	}
}

// TestProbeHandlerMissingTarget tests the probe handler with a missing target.
func TestProbeHandlerMissingTarget(t *testing.T) {
	cfg, _ := setupTest(t)
	srv := New(cfg)

	// Create a test request without a target
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	w := httptest.NewRecorder()

	// Call the handler
	srv.probeHandler(w, req)

	// Check the response
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("probeHandler() status = %v, want %v", resp.StatusCode, http.StatusBadRequest)
	}

	// Check that the response contains the expected content
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	expectedError := "'target' parameter must be specified"
	if !strings.Contains(string(body), expectedError) {
		t.Errorf("probeHandler() response = %q, does not contain %q", string(body), expectedError)
	}
}

// TestProbeHandlerInvalidPort tests the probe handler with an invalid port.
func TestProbeHandlerInvalidPort(t *testing.T) {
	cfg, _ := setupTest(t)
	srv := New(cfg)

	// Create a test request with an invalid port
	req := httptest.NewRequest(http.MethodGet, "/probe?target=example.com&port=invalid", nil)
	w := httptest.NewRecorder()

	// Call the handler
	srv.probeHandler(w, req)

	// Check the response
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("probeHandler() status = %v, want %v", resp.StatusCode, http.StatusBadRequest)
	}

	// Check that the response contains the expected content
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	expectedError := "'port' parameter must be an integer"
	if !strings.Contains(string(body), expectedError) {
		t.Errorf("probeHandler() response = %q, does not contain %q", string(body), expectedError)
	}
}

// TestProbeHandlerInvalidReverseMode tests the probe handler with an invalid reverse_mode.
func TestProbeHandlerInvalidReverseMode(t *testing.T) {
	cfg, _ := setupTest(t)
	srv := New(cfg)

	// Create a test request with an invalid reverse_mode
	req := httptest.NewRequest(http.MethodGet, "/probe?target=example.com&reverse_mode=invalid", nil)
	w := httptest.NewRecorder()

	// Call the handler
	srv.probeHandler(w, req)

	// Check the response
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("probeHandler() status = %v, want %v", resp.StatusCode, http.StatusBadRequest)
	}

	// Check that the response contains the expected content
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	expectedError := "'reverse_mode' parameter must be true or false"
	if !strings.Contains(string(body), expectedError) {
		t.Errorf("probeHandler() response = %q, does not contain %q", string(body), expectedError)
	}
}

// TestProbeHandlerInvalidBitrate tests the probe handler with an invalid bitrate.
func TestProbeHandlerInvalidBitrate(t *testing.T) {
	cfg, _ := setupTest(t)
	srv := New(cfg)

	// Create a test request with an invalid bitrate
	req := httptest.NewRequest(http.MethodGet, "/probe?target=example.com&bitrate=invalid", nil)
	w := httptest.NewRecorder()

	// Call the handler
	srv.probeHandler(w, req)

	// Check the response
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("probeHandler() status = %v, want %v", resp.StatusCode, http.StatusBadRequest)
	}

	// Check that the response contains the expected content
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	expectedError := "bitrate must provided as"
	if !strings.Contains(string(body), expectedError) {
		t.Errorf("probeHandler() response = %q, does not contain %q", string(body), expectedError)
	}
}

// TestProbeHandlerInvalidPeriod tests the probe handler with an invalid period.
func TestProbeHandlerInvalidPeriod(t *testing.T) {
	cfg, _ := setupTest(t)
	srv := New(cfg)

	// Create a test request with an invalid period
	req := httptest.NewRequest(http.MethodGet, "/probe?target=example.com&period=invalid", nil)
	w := httptest.NewRecorder()

	// Call the handler
	srv.probeHandler(w, req)

	// Check the response
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("probeHandler() status = %v, want %v", resp.StatusCode, http.StatusBadRequest)
	}

	// Check that the response contains the expected content
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	expectedError := "'period' parameter must be a duration"
	if !strings.Contains(string(body), expectedError) {
		t.Errorf("probeHandler() response = %q, does not contain %q", string(body), expectedError)
	}
}

// TestProbeHandlerInvalidTimeout tests the probe handler with an invalid timeout.
func TestProbeHandlerInvalidTimeout(t *testing.T) {
	cfg, _ := setupTest(t)
	srv := New(cfg)

	// Create a test request with an invalid timeout
	req := httptest.NewRequest(http.MethodGet, "/probe?target=example.com", nil)
	req.Header.Set("X-Prometheus-Scrape-Timeout-Seconds", "invalid")

	w := httptest.NewRecorder()

	// Call the handler
	srv.probeHandler(w, req)

	// Check the response
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("probeHandler() status = %v, want %v", resp.StatusCode, http.StatusInternalServerError)
	}

	// Check that the response contains the expected content
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	expectedError := "Failed to parse timeout from Prometheus header"
	if !strings.Contains(string(body), expectedError) {
		t.Errorf("probeHandler() response = %q, does not contain %q", string(body), expectedError)
	}
}

// TestWithLogging tests the logging middleware.
func TestWithLogging(t *testing.T) {
	cfg, _ := setupTest(t)
	srv := New(cfg)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		if _, err := w.Write([]byte("OK")); err != nil {
			t.Fatalf("Failed to write response: %v", err)
		}
	})

	// Wrap the test handler with the logging middleware
	handler := srv.withLogging(testHandler)

	// Create a test request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	// Call the handler
	handler.ServeHTTP(w, req)

	// Check the response
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("withLogging() status = %v, want %v", resp.StatusCode, http.StatusOK)
	}

	// Check that the response contains the expected content
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if string(body) != "OK" {
		t.Errorf("withLogging() response = %q, want %q", string(body), "OK")
	}
}

// TestStop tests the Stop method.
func TestStop(t *testing.T) {
	cfg, _ := setupTest(t)
	srv := New(cfg)

	// Create a test HTTP server
	srv.server = &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	}

	// Call Stop
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	err := srv.Stop(ctx)
	if err != nil {
		t.Errorf("Stop() error = %v, want nil", err)
	}
}
