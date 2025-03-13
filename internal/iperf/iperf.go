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

// Package iperf provides functionality for running iperf3 tests and parsing results.
package iperf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/edgard/iperf3_exporter/internal/validation"
)

// Config represents the configuration for an iperf3 test.
type Config struct {
	Target      string
	Port        int
	Period      time.Duration
	Timeout     time.Duration
	ReverseMode bool
	Bitrate     string
	Logger      *slog.Logger
}

// rawResult collects the result from the iperf3 run.
type rawResult struct {
	End struct {
		SumSent struct {
			Seconds       float64 `json:"seconds"`
			Bytes         float64 `json:"bytes"`
			BitsPerSecond float64 `json:"bits_per_second"`
			Retransmits   float64 `json:"retransmits"`
		} `json:"sum_sent"`
		SumReceived struct {
			Seconds       float64 `json:"seconds"`
			Bytes         float64 `json:"bytes"`
			BitsPerSecond float64 `json:"bits_per_second"`
		} `json:"sum_received"`
	} `json:"end"`
}

// Result represents the parsed result from an iperf3 test.
type Result struct {
	Success bool
	Metrics map[string]float64
	Error   error
}

// Runner defines the interface for running iperf3 tests.
type Runner interface {
	Run(ctx context.Context, cfg Config) Result
}

// runner is the implementation of the Runner interface.
type runner struct {
	logger *slog.Logger
}

// NewRunner creates a new iperf3 runner.
func NewRunner(logger *slog.Logger) Runner {
	return &runner{logger: logger}
}

// Run executes an iperf3 test with the given configuration and returns the parsed results.
func (r *runner) Run(ctx context.Context, cfg Config) Result {
	result := Result{
		Success: false,
		Metrics: make(map[string]float64),
	}

	// Validate configuration
	if err := validation.ValidateBitrate(cfg.Bitrate); err != nil {
		return Result{
			Success: false,
			Metrics: make(map[string]float64),
			Error:   fmt.Errorf("invalid bitrate: %w", err),
		}
	}

	if err := validation.ValidatePort(cfg.Port); err != nil {
		return Result{
			Success: false,
			Metrics: make(map[string]float64),
			Error:   fmt.Errorf("invalid port: %w", err),
		}
	}

	// Prepare command arguments
	iperfArgs := []string{
		"-J",                                                        // JSON output
		"-t", strconv.FormatFloat(cfg.Period.Seconds(), 'f', 0, 64), // Test duration
		"-c", cfg.Target, // Target server
		"-p", strconv.Itoa(cfg.Port), // Target port
	}

	if cfg.ReverseMode {
		iperfArgs = append(iperfArgs, "-R") // Reverse mode
	}

	if cfg.Bitrate != "" {
		iperfArgs = append(iperfArgs, "-b", cfg.Bitrate) // Bitrate limit
	}

	// Create command
	cmd := exec.CommandContext(ctx, GetIperfCmd(), iperfArgs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// Log command execution
	r.logger.Debug("Running iperf3 command",
		"target", cfg.Target,
		"port", cfg.Port,
		"period", cfg.Period,
		"reverse", cfg.ReverseMode,
		"bitrate", cfg.Bitrate,
	)

	// Execute command
	output, err := cmd.Output()
	if err != nil {
		errMsg := stderr.String()
		if errMsg != "" {
			err = fmt.Errorf("%w: %s", err, errMsg)
		}
		return Result{
			Success: false,
			Metrics: make(map[string]float64),
			Error:   fmt.Errorf("iperf3 execution failed: %w", err),
		}
	}

	// Parse result
	var raw rawResult
	if err := json.Unmarshal(output, &raw); err != nil {
		return Result{
			Success: false,
			Metrics: make(map[string]float64),
			Error:   fmt.Errorf("failed to parse iperf3 result: %w", err),
		}
	}

	// Extract metrics
	result.Success = true
	result.Metrics = map[string]float64{
		"sent_seconds":     raw.End.SumSent.Seconds,
		"sent_bytes":       raw.End.SumSent.Bytes,
		"received_seconds": raw.End.SumReceived.Seconds,
		"received_bytes":   raw.End.SumReceived.Bytes,
		"retransmits":      raw.End.SumSent.Retransmits,
	}

	r.logger.Debug("iperf3 test completed successfully",
		"target", cfg.Target,
		"sent_bps", raw.End.SumSent.BitsPerSecond,
		"received_bps", raw.End.SumReceived.BitsPerSecond,
	)

	return result
}

// GetIperfCmd returns the command name for iperf3 based on the platform.
func GetIperfCmd() string {
	// Simple cross-platform implementation using runtime detection
	if runtime.GOOS == "windows" {
		return "iperf3.exe"
	}
	return "iperf3"
}

// CheckIperf3Exists verifies that the iperf3 command exists and is executable.
func CheckIperf3Exists() error {
	_, err := exec.LookPath(GetIperfCmd())
	return err
}
