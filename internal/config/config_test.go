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

package config

import (
	"os"
	"testing"
	"time"

	"github.com/alecthomas/kingpin/v2"
)

// TestNewConfig tests that NewConfig returns a Config with default values.
func TestNewConfig(t *testing.T) {
	cfg := NewConfig()

	// Check default values
	if cfg.ListenAddress != ":9579" {
		t.Errorf("NewConfig() ListenAddress = %q, want %q", cfg.ListenAddress, ":9579")
	}

	if cfg.MetricsPath != "/metrics" {
		t.Errorf("NewConfig() MetricsPath = %q, want %q", cfg.MetricsPath, "/metrics")
	}

	if cfg.ProbePath != "/probe" {
		t.Errorf("NewConfig() ProbePath = %q, want %q", cfg.ProbePath, "/probe")
	}

	if cfg.Timeout != 30*time.Second {
		t.Errorf("NewConfig() Timeout = %v, want %v", cfg.Timeout, 30*time.Second)
	}

	if cfg.Logger != nil {
		t.Errorf("NewConfig() Logger = %v, want nil", cfg.Logger)
	}
}

// TestParseFlags tests that ParseFlags correctly parses command-line flags.
func TestParseFlags(t *testing.T) {
	// Save original os.Args and restore after test
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	// Save original kingpin.CommandLine and restore after test
	originalCommandLine := kingpin.CommandLine
	defer func() { kingpin.CommandLine = originalCommandLine }()

	// Reset kingpin.CommandLine for this test
	kingpin.CommandLine = kingpin.New(os.Args[0], "Test")

	// Test cases
	testCases := []struct {
		name          string
		args          []string
		metricsPath   string
		probePath     string
		timeout       time.Duration
		logLevel      string
		logFormat     string
		listenAddress string
	}{
		{
			name:          "default values",
			args:          []string{"app"},
			metricsPath:   "/metrics",
			probePath:     "/probe",
			timeout:       30 * time.Second,
			logLevel:      "info",
			logFormat:     "logfmt",
			listenAddress: ":9579",
		},
		{
			name:          "custom values",
			args:          []string{"app", "--web.telemetry-path=/custom-metrics", "--web.probe-path=/custom-probe", "--iperf3.timeout=10s", "--log.level=debug", "--log.format=json"},
			metricsPath:   "/custom-metrics",
			probePath:     "/custom-probe",
			timeout:       10 * time.Second,
			logLevel:      "debug",
			logFormat:     "json",
			listenAddress: ":9579",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset kingpin.CommandLine for each test case
			kingpin.CommandLine = kingpin.New(os.Args[0], "Test")

			// Set os.Args for this test case
			os.Args = tc.args

			// Parse flags
			cfg := ParseFlags()

			// Check values
			if cfg.MetricsPath != tc.metricsPath {
				t.Errorf("ParseFlags() MetricsPath = %q, want %q", cfg.MetricsPath, tc.metricsPath)
			}

			if cfg.ProbePath != tc.probePath {
				t.Errorf("ParseFlags() ProbePath = %q, want %q", cfg.ProbePath, tc.probePath)
			}

			if cfg.Timeout != tc.timeout {
				t.Errorf("ParseFlags() Timeout = %v, want %v", cfg.Timeout, tc.timeout)
			}

			// Check that logger is initialized
			if cfg.Logger == nil {
				t.Error("ParseFlags() Logger is nil, want non-nil")
			}
		})
	}
}

// TestValidate tests that Validate correctly validates the configuration.
func TestValidate(t *testing.T) {
	cfg := NewConfig()

	// Validate should always return nil for now
	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() returned error: %v", err)
	}
}

// TestLogLevelParsing tests that log levels are correctly parsed.
func TestLogLevelParsing(t *testing.T) {
	// Save original os.Args and restore after test
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	// Save original kingpin.CommandLine and restore after test
	originalCommandLine := kingpin.CommandLine
	defer func() { kingpin.CommandLine = originalCommandLine }()

	// Test cases for log levels
	testCases := []struct {
		name     string
		logLevel string
	}{
		{
			name:     "debug level",
			logLevel: "debug",
		},
		{
			name:     "info level",
			logLevel: "info",
		},
		{
			name:     "warn level",
			logLevel: "warn",
		},
		{
			name:     "error level",
			logLevel: "error",
		},
		{
			name:     "invalid level (defaults to info)",
			logLevel: "invalid",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset kingpin.CommandLine for each test case
			kingpin.CommandLine = kingpin.New(os.Args[0], "Test")

			// Set os.Args for this test case
			os.Args = []string{"app", "--log.level=" + tc.logLevel}

			// Parse flags
			cfg := ParseFlags()

			// Check that logger is initialized
			if cfg.Logger == nil {
				t.Error("ParseFlags() Logger is nil, want non-nil")
			}
			// We can't easily check the log level directly, but we can verify that
			// the logger was created without errors
		})
	}
}

// TestLogFormatParsing tests that log formats are correctly parsed.
func TestLogFormatParsing(t *testing.T) {
	// Save original os.Args and restore after test
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	// Save original kingpin.CommandLine and restore after test
	originalCommandLine := kingpin.CommandLine
	defer func() { kingpin.CommandLine = originalCommandLine }()

	// Test cases for log formats
	testCases := []struct {
		name      string
		logFormat string
	}{
		{
			name:      "logfmt format",
			logFormat: "logfmt",
		},
		{
			name:      "json format",
			logFormat: "json",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset kingpin.CommandLine for each test case
			kingpin.CommandLine = kingpin.New(os.Args[0], "Test")

			// Set os.Args for this test case
			os.Args = []string{"app", "--log.format=" + tc.logFormat}

			// Parse flags
			cfg := ParseFlags()

			// Check that logger is initialized
			if cfg.Logger == nil {
				t.Error("ParseFlags() Logger is nil, want non-nil")
			}
			// We can't easily check the log format directly, but we can verify that
			// the logger was created without errors
		})
	}
}
