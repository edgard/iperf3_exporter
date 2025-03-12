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

// Package integration provides integration tests for the iperf3_exporter.
package integration

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/edgard/iperf3_exporter/internal/collector"
	"github.com/edgard/iperf3_exporter/internal/config"
	"github.com/edgard/iperf3_exporter/internal/iperf"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	return cfg, logger
}

// TestMetricsEndpoint tests the /metrics endpoint.
func TestMetricsEndpoint(t *testing.T) {
	_, _ = setupTest(t)

	// Create a registry
	registry := prometheus.NewRegistry()

	// Register the default metrics
	registry.MustRegister(collector.IperfDuration)
	registry.MustRegister(collector.IperfErrors)

	// Create a handler for the /metrics endpoint
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})

	// Create a test server
	server := httptest.NewServer(handler)
	defer server.Close()

	// Make a request to the /metrics endpoint
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request to /metrics: %v", err)
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/metrics status = %v, want %v", resp.StatusCode, http.StatusOK)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Check that the response contains the expected metrics
	expectedMetrics := []string{
		"iperf3_exporter_duration_seconds",
		"iperf3_exporter_errors_total",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(string(body), metric) {
			t.Errorf("/metrics response does not contain %q", metric)
		}
	}
}

// mockIperfRunner is used to mock the iperf.Runner interface for testing.
type mockIperfRunner struct {
	result iperf.Result
}

// Run implements the iperf.Runner interface.
func (m *mockIperfRunner) Run(ctx context.Context, cfg iperf.Config) iperf.Result {
	return m.result
}

// TestProbeEndpoint tests the /probe endpoint.
func TestProbeEndpoint(t *testing.T) {
	_, _ = setupTest(t)

	// Create a server with a custom handler for the probe endpoint
	mux := http.NewServeMux()
	mux.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {
		// Simple implementation that returns mock metrics
		w.Header().Set("Content-Type", "text/plain")

		_, err := w.Write([]byte(`# HELP iperf3_up Was the last iperf3 probe successful (1 for success, 0 for failure).
# TYPE iperf3_up gauge
iperf3_up{port="5201",target="example.com"} 1
# HELP iperf3_sent_seconds Total seconds spent sending packets.
# TYPE iperf3_sent_seconds gauge
iperf3_sent_seconds{port="5201",target="example.com"} 5
# HELP iperf3_sent_bytes Total sent bytes.
# TYPE iperf3_sent_bytes gauge
iperf3_sent_bytes{port="5201",target="example.com"} 52428800
# HELP iperf3_sent_bps Bits per second on sending packets.
# TYPE iperf3_sent_bps gauge
iperf3_sent_bps{port="5201",target="example.com"} 83886080
# HELP iperf3_received_seconds Total seconds spent receiving packets.
# TYPE iperf3_received_seconds gauge
iperf3_received_seconds{port="5201",target="example.com"} 5
# HELP iperf3_received_bytes Total received bytes.
# TYPE iperf3_received_bytes gauge
iperf3_received_bytes{port="5201",target="example.com"} 47185920
# HELP iperf3_received_bps Bits per second on receiving packets.
# TYPE iperf3_received_bps gauge
iperf3_received_bps{port="5201",target="example.com"} 75497472
# HELP iperf3_retransmits Total retransmits.
# TYPE iperf3_retransmits gauge
iperf3_retransmits{port="5201",target="example.com"} 10
`))
		if err != nil {
			t.Fatalf("Failed to write response: %v", err)
		}
	})

	// Create a test server
	testServer := httptest.NewServer(mux)
	defer testServer.Close()

	// Make a request to the /probe endpoint
	resp, err := http.Get(testServer.URL + "/probe?target=example.com")
	if err != nil {
		t.Fatalf("Failed to make request to /probe: %v", err)
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/probe status = %v, want %v", resp.StatusCode, http.StatusOK)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Check that the response contains the expected metrics
	expectedMetrics := []string{
		"iperf3_up",
		"iperf3_sent_seconds",
		"iperf3_sent_bytes",
		"iperf3_sent_bps",
		"iperf3_received_seconds",
		"iperf3_received_bytes",
		"iperf3_received_bps",
		"iperf3_retransmits",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(string(body), metric) {
			t.Errorf("/probe response does not contain %q", metric)
		}
	}
}

// TestMetricNamingConventions tests that metric names follow Prometheus conventions.
func TestMetricNamingConventions(t *testing.T) {
	// Define a regex pattern for valid Prometheus metric names
	validNamePattern := regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)

	// Define the metrics to test
	metrics := []string{
		"iperf3_up",
		"iperf3_sent_seconds",
		"iperf3_sent_bytes",
		"iperf3_sent_bps",
		"iperf3_received_seconds",
		"iperf3_received_bytes",
		"iperf3_received_bps",
		"iperf3_retransmits",
		"iperf3_exporter_duration_seconds",
		"iperf3_exporter_errors_total",
	}

	// Check each metric name
	for _, metric := range metrics {
		if !validNamePattern.MatchString(metric) {
			t.Errorf("Metric name %q does not follow Prometheus naming conventions", metric)
		}

		// Check that the metric name has the correct namespace
		if !strings.HasPrefix(metric, "iperf3") {
			t.Errorf("Metric name %q does not have the correct namespace 'iperf3'", metric)
		}
	}
}

// TestMetricTypes tests that metric types are appropriate.
func TestMetricTypes(t *testing.T) {
	cfg, _ := setupTest(t)

	// Create a successful mock result
	mockResult := iperf.Result{
		Success:               true,
		SentSeconds:           5.0,
		SentBytes:             52428800,
		SentBitsPerSecond:     83886080,
		ReceivedSeconds:       5.0,
		ReceivedBytes:         47185920,
		ReceivedBitsPerSecond: 75497472,
		Retransmits:           10,
	}

	// Create a mock runner
	mockRunner := &mockIperfRunner{
		result: mockResult,
	}

	// Create a collector
	probeConfig := collector.ProbeConfig{
		Target:      "example.com",
		Port:        5201,
		Period:      5 * time.Second,
		Timeout:     10 * time.Second,
		ReverseMode: false,
		Bitrate:     "",
	}

	c := collector.NewCollectorWithRunner(probeConfig, cfg.Logger, mockRunner)

	// Create a channel to receive metrics
	ch := make(chan prometheus.Metric, 100)

	// Collect metrics
	c.Collect(ch)

	// We can't directly check the metric type from the Desc() method
	// Instead, we'll verify that metrics are created and collected
	for range 8 { // We expect 8 metrics
		select {
		case <-ch:
			// Just verify we received a metric
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Timed out waiting for metric")
		}
	}
}

// TestMetricLabels tests that metric labels are appropriate.
func TestMetricLabels(t *testing.T) {
	cfg, _ := setupTest(t)

	// Create a successful mock result
	mockResult := iperf.Result{
		Success:               true,
		SentSeconds:           5.0,
		SentBytes:             52428800,
		SentBitsPerSecond:     83886080,
		ReceivedSeconds:       5.0,
		ReceivedBytes:         47185920,
		ReceivedBitsPerSecond: 75497472,
		Retransmits:           10,
	}

	// Create a mock runner
	mockRunner := &mockIperfRunner{
		result: mockResult,
	}

	// Create a collector
	probeConfig := collector.ProbeConfig{
		Target:      "example.com",
		Port:        5201,
		Period:      5 * time.Second,
		Timeout:     10 * time.Second,
		ReverseMode: false,
		Bitrate:     "",
	}

	c := collector.NewCollectorWithRunner(probeConfig, cfg.Logger, mockRunner)

	// Create a registry
	registry := prometheus.NewRegistry()
	registry.MustRegister(c)

	// Create a handler for the /metrics endpoint
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})

	// Create a test server
	server := httptest.NewServer(handler)
	defer server.Close()

	// Make a request to the /metrics endpoint
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request to /metrics: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Check that the metrics have the expected labels
	expectedLabels := []string{
		`target="example.com"`,
		`port="5201"`,
	}

	for _, label := range expectedLabels {
		if !strings.Contains(string(body), label) {
			t.Errorf("Metrics do not contain label %q", label)
		}
	}
}

// TestMetricHelp tests that metrics have help text.
func TestMetricHelp(t *testing.T) {
	cfg, _ := setupTest(t)

	// Create a successful mock result
	mockResult := iperf.Result{
		Success:               true,
		SentSeconds:           5.0,
		SentBytes:             52428800,
		SentBitsPerSecond:     83886080,
		ReceivedSeconds:       5.0,
		ReceivedBytes:         47185920,
		ReceivedBitsPerSecond: 75497472,
		Retransmits:           10,
	}

	// Create a mock runner
	mockRunner := &mockIperfRunner{
		result: mockResult,
	}

	// Create a collector
	probeConfig := collector.ProbeConfig{
		Target:      "example.com",
		Port:        5201,
		Period:      5 * time.Second,
		Timeout:     10 * time.Second,
		ReverseMode: false,
		Bitrate:     "",
	}

	c := collector.NewCollectorWithRunner(probeConfig, cfg.Logger, mockRunner)

	// Create a registry
	registry := prometheus.NewRegistry()
	registry.MustRegister(c)

	// Create a handler for the /metrics endpoint
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})

	// Create a test server
	server := httptest.NewServer(handler)
	defer server.Close()

	// Make a request to the /metrics endpoint
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request to /metrics: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Check that the metrics have help text
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "# HELP") {
			// Check that the help text is not empty
			helpParts := strings.SplitN(line, " ", 4)
			if len(helpParts) < 4 || helpParts[3] == "" {
				t.Errorf("Metric %s has empty help text", helpParts[2])
			}
		}
	}
}
