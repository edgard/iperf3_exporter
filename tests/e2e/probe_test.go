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

// Package e2e contains end-to-end tests for the iperf3_exporter.
package e2e

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/edgard/iperf3_exporter/internal/collector"
	"github.com/edgard/iperf3_exporter/internal/iperf"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MockRunner implements the iperf.Runner interface for testing
type MockRunner struct {
	Result iperf.Result
}

// Run implements the iperf.Runner interface
func (r *MockRunner) Run(ctx context.Context, cfg iperf.Config) iperf.Result {
	return r.Result
}

// TestProbeEndpoint tests the /probe endpoint of the iperf3_exporter.
func TestProbeEndpoint(t *testing.T) {
	// Create a mock runner with predefined results
	mockRunner := &MockRunner{
		Result: iperf.Result{
			Success:               true,
			SentSeconds:           5.0,
			SentBytes:             5242880,
			SentBitsPerSecond:     8388608,
			ReceivedSeconds:       5.0,
			ReceivedBytes:         5242880,
			ReceivedBitsPerSecond: 8388608,
			Retransmits:           0,
		},
	}

	// Create a test server with a handler that mimics the probe endpoint
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse query parameters
		target := r.URL.Query().Get("target")
		if target == "" {
			http.Error(w, "'target' parameter must be specified", http.StatusBadRequest)
			return
		}

		var targetPort int
		port := r.URL.Query().Get("port")
		if port != "" {
			var err error
			targetPort, err = strconv.Atoi(port)
			if err != nil {
				http.Error(w, "'port' parameter must be an integer", http.StatusBadRequest)
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
				http.Error(w, "'reverse_mode' parameter must be true or false", http.StatusBadRequest)
				return
			}
		}

		bitrate := r.URL.Query().Get("bitrate")
		if bitrate != "" && !iperf.ValidateBitrate(bitrate) {
			http.Error(w, "invalid bitrate format", http.StatusBadRequest)
			return
		}

		var runPeriod time.Duration
		period := r.URL.Query().Get("period")
		if period != "" {
			var err error
			runPeriod, err = time.ParseDuration(period)
			if err != nil {
				http.Error(w, "'period' parameter must be a duration", http.StatusBadRequest)
				return
			}
		}

		if runPeriod.Seconds() == 0 {
			runPeriod = time.Second * 5
		}

		bind := r.URL.Query().Get("bind")

		// Create a collector with the mock runner
		registry := prometheus.NewRegistry()
		probeConfig := collector.ProbeConfig{
			Target:      target,
			Port:        targetPort,
			Period:      runPeriod,
			Timeout:     30 * time.Second,
			ReverseMode: reverseMode,
			Bitrate:     bitrate,
			Bind:        bind,
		}
		c := collector.NewCollectorWithRunner(probeConfig, slog.Default(), mockRunner)
		registry.MustRegister(c)

		// Serve metrics
		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	}))
	defer ts.Close()

	// Test case 1: Successful request with default parameters
	t.Run("SuccessfulRequest", func(t *testing.T) {
		// Make a request to the test server
		resp, err := http.Get(ts.URL + "?target=test.example.com")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status OK, got %v", resp.Status)
		}

		// Parse and verify metrics
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}

		// Convert body to string for easier inspection
		bodyStr := string(body)

		// Verify metrics contain expected values using regex patterns
		expectedPatterns := []*regexp.Regexp{
			regexp.MustCompile(`iperf3_up\{port="5201".*target="test.example.com"\} 1`),
			regexp.MustCompile(`iperf3_sent_seconds\{port="5201".*target="test.example.com"\} 5`),
			regexp.MustCompile(`iperf3_sent_bytes\{port="5201".*target="test.example.com"\} 5\.24288e\+06`),
			regexp.MustCompile(`iperf3_received_seconds\{port="5201".*target="test.example.com"\} 5`),
			regexp.MustCompile(`iperf3_received_bytes\{port="5201".*target="test.example.com"\} 5\.24288e\+06`),
			regexp.MustCompile(`iperf3_retransmits\{port="5201".*target="test.example.com"\} 0`),
		}

		for _, pattern := range expectedPatterns {
			if !pattern.MatchString(bodyStr) {
				t.Errorf("Expected metric matching pattern %q not found in response", pattern.String())
			}
		}
	})

	// Test case 2: Missing target parameter
	t.Run("MissingTarget", func(t *testing.T) {
		// Make a request without a target
		resp, err := http.Get(ts.URL)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		// Check response status
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("Expected status Bad Request, got %v", resp.Status)
		}

		// Parse and verify error message
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}

		expectedError := "'target' parameter must be specified"
		if !strings.Contains(string(body), expectedError) {
			t.Errorf("Expected error message %q not found in response", expectedError)
		}
	})

	// Test case 3: Custom port parameter
	t.Run("CustomPort", func(t *testing.T) {
		// Make a request with a custom port
		resp, err := http.Get(ts.URL + "?target=test.example.com&port=9999")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status OK, got %v", resp.Status)
		}

		// Parse and verify metrics
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}

		// Verify metrics contain expected port using regex
		expectedPattern := regexp.MustCompile(`iperf3_up\{port="9999".*target="test.example.com"\} 1`)
		if !expectedPattern.MatchString(string(body)) {
			t.Errorf("Expected metric matching pattern %q not found in response", expectedPattern.String())
		}
	})

	// Test case 4: Reverse mode parameter
	t.Run("ReverseMode", func(t *testing.T) {
		// Make a request with reverse mode enabled
		resp, err := http.Get(ts.URL + "?target=test.example.com&reverse_mode=true")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status OK, got %v", resp.Status)
		}

		// Parse and verify metrics
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}

		// Verify the request was processed with reverse mode
		// We can't directly verify the reverse mode parameter in the response,
		// but we can check that the metrics are present and have the expected values
		expectedPattern := regexp.MustCompile(`iperf3_up\{port="5201".*target="test.example.com"\} 1`)
		if !expectedPattern.MatchString(string(body)) {
			t.Errorf("Expected metric matching pattern %q not found in response", expectedPattern.String())
		}
	})

	// Test case 5: Bitrate parameter
	t.Run("Bitrate", func(t *testing.T) {
		// Make a request with a custom bitrate
		resp, err := http.Get(ts.URL + "?target=test.example.com&bitrate=100M")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status OK, got %v", resp.Status)
		}

		// Parse and verify metrics
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}

		// Verify the request was processed with the bitrate parameter
		// We can't directly verify the bitrate parameter in the response,
		// but we can check that the metrics are present and have the expected values
		expectedPattern := regexp.MustCompile(`iperf3_up\{port="5201".*target="test.example.com"\} 1`)
		if !expectedPattern.MatchString(string(body)) {
			t.Errorf("Expected metric matching pattern %q not found in response", expectedPattern.String())
		}
	})

	// Test case 6: Period parameter
	t.Run("Period", func(t *testing.T) {
		// Make a request with a custom period
		resp, err := http.Get(ts.URL + "?target=test.example.com&period=10s")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status OK, got %v", resp.Status)
		}

		// Parse and verify metrics
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}

		// Verify the request was processed with the period parameter
		// We can't directly verify the period parameter in the response,
		// but we can check that the metrics are present and have the expected values
		expectedPattern := regexp.MustCompile(`iperf3_up\{port="5201".*target="test.example.com"\} 1`)
		if !expectedPattern.MatchString(string(body)) {
			t.Errorf("Expected metric matching pattern %q not found in response", expectedPattern.String())
		}
	})

	// Test case 7: Failed iperf3 test
	t.Run("FailedTest", func(t *testing.T) {
		// Create a mock runner that returns a failure
		failedRunner := &MockRunner{
			Result: iperf.Result{
				Success: false,
				Error:   fmt.Errorf("iperf3 test failed"),
			},
		}

		// Create a test server with the failed runner
		failedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			target := r.URL.Query().Get("target")
			if target == "" {
				http.Error(w, "'target' parameter must be specified", http.StatusBadRequest)
				return
			}

			registry := prometheus.NewRegistry()
			probeConfig := collector.ProbeConfig{
				Target:  target,
				Port:    5201,
				Period:  5 * time.Second,
				Timeout: 30 * time.Second,
			}
			c := collector.NewCollectorWithRunner(probeConfig, slog.Default(), failedRunner)
			registry.MustRegister(c)

			h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
			h.ServeHTTP(w, r)
		}))
		defer failedServer.Close()

		// Make a request to the failed server
		resp, err := http.Get(failedServer.URL + "?target=test.example.com")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status OK, got %v", resp.Status)
		}

		// Parse and verify metrics
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}

		// Verify metrics indicate failure using regex
		expectedPattern := regexp.MustCompile(`iperf3_up\{port="5201".*target="test.example.com"\} 0`)
		if !expectedPattern.MatchString(string(body)) {
			t.Errorf("Expected metric matching pattern %q not found in response", expectedPattern.String())
		}
	})
}
