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

package collector

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/edgard/iperf3_exporter/internal/iperf"
	"github.com/prometheus/client_golang/prometheus"
)

// mockIperfRunner is used to mock the iperf.Runner interface for testing.
type mockIperfRunner struct {
	result iperf.Result
}

// Run implements the iperf.Runner interface.
func (m *mockIperfRunner) Run(ctx context.Context, cfg iperf.Config) iperf.Result {
	return m.result
}

// setupTest initializes common test resources.
func setupTest(t *testing.T) (*slog.Logger, *prometheus.Registry) {
	t.Helper()

	// Create a test logger that writes to stderr
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	// Create a new registry for each test to ensure isolation
	registry := prometheus.NewRegistry()

	return logger, registry
}

// TestCollectorRegistration tests that the collector can be registered with Prometheus.
func TestCollectorRegistration(t *testing.T) {
	logger, registry := setupTest(t)

	config := ProbeConfig{
		Target:      "example.com",
		Port:        5201,
		Period:      5 * time.Second,
		Timeout:     10 * time.Second,
		ReverseMode: false,
		Bitrate:     "",
	}

	collector := NewCollector(config, logger)

	err := registry.Register(collector)
	if err != nil {
		t.Fatalf("Failed to register collector: %v", err)
	}

	// Verify that the collector can be unregistered
	if !registry.Unregister(collector) {
		t.Fatal("Failed to unregister collector")
	}
}

// TestCollectorDescribe tests that the collector correctly describes its metrics.
func TestCollectorDescribe(t *testing.T) {
	logger, _ := setupTest(t)

	config := ProbeConfig{
		Target:      "example.com",
		Port:        5201,
		Period:      5 * time.Second,
		Timeout:     10 * time.Second,
		ReverseMode: false,
		Bitrate:     "",
	}

	collector := NewCollector(config, logger)

	// Create a channel to receive metric descriptions
	ch := make(chan *prometheus.Desc, 10)

	// Call Describe and count the number of metrics described
	collector.Describe(ch)

	// We expect 8 metrics to be described
	expectedMetrics := 8
	actualMetrics := 0

	// Count the metrics
	for {
		select {
		case <-ch:
			actualMetrics++
		default:
			// Channel is empty
			if actualMetrics != expectedMetrics {
				t.Errorf("Expected %d metrics, got %d", expectedMetrics, actualMetrics)
			}

			return
		}
	}
}

// TestCollectorCollect tests the collection of metrics with a successful iperf run.
func TestCollectorCollect(t *testing.T) {
	logger, registry := setupTest(t)

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

	// Create a collector with the mock runner
	config := ProbeConfig{
		Target:      "example.com",
		Port:        5201,
		Period:      5 * time.Second,
		Timeout:     10 * time.Second,
		ReverseMode: false,
		Bitrate:     "",
	}

	collector := NewCollectorWithRunner(config, logger, mockRunner)

	// Register the collector
	err := registry.Register(collector)
	if err != nil {
		t.Fatalf("Failed to register collector: %v", err)
	}

	// Create a channel to receive metrics
	ch := make(chan prometheus.Metric, 10)

	// Collect metrics
	collector.Collect(ch)

	// Verify that 8 metrics were collected
	expectedMetrics := 8
	actualMetrics := 0

	// Count the metrics
	for {
		select {
		case <-ch:
			actualMetrics++
		default:
			// Channel is empty
			if actualMetrics != expectedMetrics {
				t.Errorf("Expected %d metrics, got %d", expectedMetrics, actualMetrics)
			}

			return
		}
	}
}

// TestCollectorCollectFailure tests the collection of metrics with a failed iperf run.
func TestCollectorCollectFailure(t *testing.T) {
	logger, registry := setupTest(t)

	// Create a failed mock result
	mockResult := iperf.Result{
		Success: false,
		Error:   errors.New("mock error"),
	}

	// Create a mock runner
	mockRunner := &mockIperfRunner{
		result: mockResult,
	}

	// Create a collector with the mock runner
	config := ProbeConfig{
		Target:      "example.com",
		Port:        5201,
		Period:      5 * time.Second,
		Timeout:     10 * time.Second,
		ReverseMode: false,
		Bitrate:     "",
	}

	collector := NewCollectorWithRunner(config, logger, mockRunner)

	// Register the collector
	err := registry.Register(collector)
	if err != nil {
		t.Fatalf("Failed to register collector: %v", err)
	}

	// Create a channel to receive metrics
	ch := make(chan prometheus.Metric, 10)

	// Collect metrics
	collector.Collect(ch)

	// Verify that 8 metrics were collected
	expectedMetrics := 8
	actualMetrics := 0

	// Count the metrics
	for {
		select {
		case <-ch:
			actualMetrics++
		default:
			// Channel is empty
			if actualMetrics != expectedMetrics {
				t.Errorf("Expected %d metrics, got %d", expectedMetrics, actualMetrics)
			}

			return
		}
	}
}

// TestCollectorConcurrency tests that the collector can handle concurrent scrapes.
func TestCollectorConcurrency(t *testing.T) {
	logger, registry := setupTest(t)

	// Create a mock result that takes some time to complete
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

	// Create a mock runner that simulates a delay
	mockRunner := &mockIperfRunner{
		result: mockResult,
	}

	// Create a collector with the mock runner
	config := ProbeConfig{
		Target:      "example.com",
		Port:        5201,
		Period:      5 * time.Second,
		Timeout:     10 * time.Second,
		ReverseMode: false,
		Bitrate:     "",
	}

	collector := NewCollectorWithRunner(config, logger, mockRunner)

	// Register the collector
	err := registry.Register(collector)
	if err != nil {
		t.Fatalf("Failed to register collector: %v", err)
	}

	// Number of concurrent scrapes
	concurrentScrapes := 5

	var wg sync.WaitGroup

	wg.Add(concurrentScrapes)

	// Run concurrent scrapes
	for range concurrentScrapes {
		go func() {
			defer wg.Done()

			// Create a channel to receive metrics
			ch := make(chan prometheus.Metric, 10)

			// Collect metrics
			collector.Collect(ch)

			// Verify that 8 metrics were collected
			expectedMetrics := 8
			actualMetrics := 0

			// Count the metrics
			for {
				select {
				case <-ch:
					actualMetrics++
				default:
					// Channel is empty
					if actualMetrics != expectedMetrics {
						t.Errorf("Expected %d metrics, got %d", expectedMetrics, actualMetrics)
					}

					return
				}
			}
		}()
	}

	// Wait for all scrapes to complete
	wg.Wait()
}

// TestMetricNamingConventions tests that metric names follow Prometheus conventions.
func TestMetricNamingConventions(t *testing.T) {
	logger, _ := setupTest(t)

	config := ProbeConfig{
		Target:      "example.com",
		Port:        5201,
		Period:      5 * time.Second,
		Timeout:     10 * time.Second,
		ReverseMode: false,
		Bitrate:     "",
	}

	collector := NewCollector(config, logger)

	// Create a channel to receive metric descriptions
	ch := make(chan *prometheus.Desc, 10)

	// Call Describe to get metric descriptions
	collector.Describe(ch)

	// Define a regex pattern for valid Prometheus metric names
	// Format: namespace_subsystem_name
	validNamePattern := regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)

	// Check each metric name
	for {
		select {
		case desc := <-ch:
			// Extract the metric name from the description
			// This is a bit hacky but works for testing
			descStr := desc.String()

			// Find the metric name in the description string
			// Format: Desc{fqName: "metric_name", ...}
			nameMatch := regexp.MustCompile(`fqName: "([^"]+)"`).FindStringSubmatch(descStr)
			if len(nameMatch) < 2 {
				t.Errorf("Failed to extract metric name from description: %s", descStr)

				continue
			}

			metricName := nameMatch[1]

			// Verify that the metric name follows Prometheus conventions
			if !validNamePattern.MatchString(metricName) {
				t.Errorf("Metric name '%s' does not follow Prometheus naming conventions", metricName)
			}

			// Verify that the metric name has the correct namespace
			if !strings.HasPrefix(metricName, namespace) {
				t.Errorf("Metric name '%s' does not have the correct namespace '%s'", metricName, namespace)
			}
		default:
			// Channel is empty
			return
		}
	}
}
