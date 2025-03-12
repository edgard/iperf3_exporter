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
	"regexp"
	"strconv"
	"time"
)

// execCommand is a variable that allows tests to mock exec.Command
var execCommand = exec.Command

// execCommandContext is a variable that allows tests to mock exec.CommandContext
var execCommandContext = exec.CommandContext

// ResetExecCommand resets the execCommand variables to the default implementation
func ResetExecCommand() {
	execCommand = exec.Command
	execCommandContext = exec.CommandContext
}

// Runner defines the interface for running iperf3 tests.
type Runner interface {
	Run(ctx context.Context, cfg Config) Result
}

// DefaultRunner is the default implementation of the Runner interface.
type DefaultRunner struct {
	Logger *slog.Logger
}

// NewRunner creates a new default iperf3 runner.
func NewRunner(logger *slog.Logger) Runner {
	return &DefaultRunner{
		Logger: logger,
	}
}

// Result represents the parsed result from an iperf3 test.
type Result struct {
	Success               bool
	SentSeconds           float64
	SentBytes             float64
	SentBitsPerSecond     float64
	ReceivedSeconds       float64
	ReceivedBytes         float64
	ReceivedBitsPerSecond float64
	Retransmits           float64
	Error                 error
}

// rawResult collects the partial result from the iperf3 run.
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

var bitratePattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?([KMG])?(\/[0-9]+)?$`)

// ValidateBitrate validates the bitrate format.
func ValidateBitrate(bitrate string) bool {
	if bitrate == "" {
		return true
	}

	return bitratePattern.MatchString(bitrate)
}

// Run executes an iperf3 test with the given configuration and returns the parsed results.
// This is a convenience function that uses the DefaultRunner.
func Run(ctx context.Context, cfg Config) Result {
	runner := NewRunner(cfg.Logger)

	return runner.Run(ctx, cfg)
}

// Run executes an iperf3 test with the given configuration and returns the parsed results.
func (r *DefaultRunner) Run(ctx context.Context, cfg Config) Result {
	// Create a result with default values
	result := Result{
		Success: false,
	}

	// Validate bitrate if provided
	if cfg.Bitrate != "" && !ValidateBitrate(cfg.Bitrate) {
		result.Error = fmt.Errorf("invalid bitrate format: %s", cfg.Bitrate)
		cfg.Logger.Error("Invalid bitrate format", "bitrate", cfg.Bitrate)

		return result
	}

	// Prepare iperf3 command arguments
	iperfArgs := []string{
		"-J",
		"-t", strconv.FormatFloat(cfg.Period.Seconds(), 'f', 0, 64),
		"-c", cfg.Target,
		"-p", strconv.Itoa(cfg.Port),
	}

	if cfg.ReverseMode {
		iperfArgs = append(iperfArgs, "-R")
	}

	if cfg.Bitrate != "" {
		iperfArgs = append(iperfArgs, "-b", cfg.Bitrate)
	}

	// Create command with context
	// #nosec G204 - GetIperfCmd returns a hardcoded string and iperfArgs are validated
	var cmd *exec.Cmd
	if ctx != nil {
		// Use the mockable execCommandContext for context-aware commands
		cmd = execCommandContext(ctx, GetIperfCmd(), iperfArgs...)
	} else {
		cmd = execCommand(GetIperfCmd(), iperfArgs...)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// Execute the command
	cfg.Logger.Debug("Running iperf3 command",
		"target", cfg.Target,
		"port", cfg.Port,
		"period", cfg.Period,
		"reverse", cfg.ReverseMode,
		"bitrate", cfg.Bitrate,
	)

	out, err := cmd.Output()
	if err != nil {
		stderrOutput := stderr.String()
		if stderrOutput != "" {
			cfg.Logger.Error("Failed to run iperf3",
				"err", err,
				"stderr", stderrOutput,
			)

			result.Error = fmt.Errorf("iperf3 execution failed: %w: %s", err, stderrOutput)
		} else {
			cfg.Logger.Error("Failed to run iperf3",
				"err", err,
			)

			result.Error = fmt.Errorf("iperf3 execution failed: %w", err)
		}

		return result
	}

	// Parse the JSON output
	var raw rawResult
	if err := json.Unmarshal(out, &raw); err != nil {
		cfg.Logger.Error("Failed to parse iperf3 result",
			"err", err,
		)

		result.Error = fmt.Errorf("failed to parse iperf3 result: %w", err)

		return result
	}

	// Populate the result
	result.Success = true
	result.SentSeconds = raw.End.SumSent.Seconds
	result.SentBytes = raw.End.SumSent.Bytes
	result.SentBitsPerSecond = raw.End.SumSent.BitsPerSecond
	result.ReceivedSeconds = raw.End.SumReceived.Seconds
	result.ReceivedBytes = raw.End.SumReceived.Bytes
	result.ReceivedBitsPerSecond = raw.End.SumReceived.BitsPerSecond
	result.Retransmits = raw.End.SumSent.Retransmits

	cfg.Logger.Debug("iperf3 test completed successfully",
		"target", cfg.Target,
		"sent_bps", result.SentBitsPerSecond,
		"received_bps", result.ReceivedBitsPerSecond,
	)

	return result
}

// CheckIperf3Exists verifies that the iperf3 command exists and is executable.
func CheckIperf3Exists() error {
	_, err := exec.LookPath(GetIperfCmd())

	return err
}
