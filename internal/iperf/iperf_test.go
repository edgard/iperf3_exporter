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

package iperf

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/exec"
	"testing"
	"time"
)

// setupTest initializes common test resources.
func setupTest(t *testing.T) *slog.Logger {
	t.Helper()

	// Create a test logger that writes to stderr
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	return logger
}

// TestValidateBitrate tests the bitrate validation function.
func TestValidateBitrate(t *testing.T) {
	testCases := []struct {
		name     string
		bitrate  string
		expected bool
	}{
		{
			name:     "empty string",
			bitrate:  "",
			expected: true,
		},
		{
			name:     "simple number",
			bitrate:  "100",
			expected: true,
		},
		{
			name:     "decimal number",
			bitrate:  "10.5",
			expected: true,
		},
		{
			name:     "kilobits",
			bitrate:  "100K",
			expected: true,
		},
		{
			name:     "megabits",
			bitrate:  "10M",
			expected: true,
		},
		{
			name:     "gigabits",
			bitrate:  "1G",
			expected: true,
		},
		{
			name:     "burst mode",
			bitrate:  "10M/100",
			expected: true,
		},
		{
			name:     "invalid unit",
			bitrate:  "10X",
			expected: false,
		},
		{
			name:     "invalid format",
			bitrate:  "10M/",
			expected: false,
		},
		{
			name:     "invalid characters",
			bitrate:  "10M-100",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ValidateBitrate(tc.bitrate)
			if result != tc.expected {
				t.Errorf("ValidateBitrate(%q) = %v, want %v", tc.bitrate, result, tc.expected)
			}
		})
	}
}

// TestGetIperfCmd tests that the correct iperf command is returned for the platform.
func TestGetIperfCmd(t *testing.T) {
	cmd := GetIperfCmd()
	if cmd == "" {
		t.Error("GetIperfCmd() returned empty string")
	}
}

// TestCheckIperf3Exists tests that the iperf3 command exists.
func TestCheckIperf3Exists(t *testing.T) {
	// Skip this test if iperf3 is not installed
	if _, err := exec.LookPath(GetIperfCmd()); err != nil {
		t.Skipf("Skipping test because iperf3 is not installed: %v", err)
	}

	err := CheckIperf3Exists()
	if err != nil {
		t.Errorf("CheckIperf3Exists() returned error: %v", err)
	}
}

// Use ResetExecCommand from the main package to reset the execCommand variable

// TestRunWithMockCommand tests the Run function with a mocked command.
func TestRunWithMockCommand(t *testing.T) {
	logger := setupTest(t)

	// Create a mock exec.Command function
	defer ResetExecCommand()

	// Create a sample iperf3 JSON output
	sampleOutput := `{
		"end": {
			"sum_sent": {
				"seconds": 5.0,
				"bytes": 52428800,
				"bits_per_second": 83886080,
				"retransmits": 10
			},
			"sum_received": {
				"seconds": 5.0,
				"bytes": 47185920,
				"bits_per_second": 75497472
			}
		}
	}`

	// Mock both execCommand and execCommandContext functions
	execCommand = func(command string, args ...string) *exec.Cmd {
		// Instead of running a subprocess, create a fake command that just returns our sample output
		cmd := exec.Command("echo", sampleOutput)
		return cmd
	}

	execCommandContext = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		// Use the same mock for context-aware commands
		cmd := exec.Command("echo", sampleOutput)
		return cmd
	}

	// Run the iperf test
	cfg := Config{
		Target:      "example.com",
		Port:        5201,
		Period:      5 * time.Second,
		Timeout:     10 * time.Second,
		ReverseMode: false,
		Bitrate:     "",
		Logger:      logger,
	}

	result := Run(t.Context(), cfg)

	// Verify the result
	if !result.Success {
		t.Errorf("Run() returned failure, expected success: %v", result.Error)
	}

	// Check the parsed values
	if result.SentSeconds != 5.0 {
		t.Errorf("Run() SentSeconds = %f, want 5.0", result.SentSeconds)
	}

	if result.SentBytes != 52428800 {
		t.Errorf("Run() SentBytes = %f, want 52428800", result.SentBytes)
	}

	if result.SentBitsPerSecond != 83886080 {
		t.Errorf("Run() SentBitsPerSecond = %f, want 83886080", result.SentBitsPerSecond)
	}

	if result.ReceivedSeconds != 5.0 {
		t.Errorf("Run() ReceivedSeconds = %f, want 5.0", result.ReceivedSeconds)
	}

	if result.ReceivedBytes != 47185920 {
		t.Errorf("Run() ReceivedBytes = %f, want 47185920", result.ReceivedBytes)
	}

	if result.ReceivedBitsPerSecond != 75497472 {
		t.Errorf("Run() ReceivedBitsPerSecond = %f, want 75497472", result.ReceivedBitsPerSecond)
	}

	if result.Retransmits != 10 {
		t.Errorf("Run() Retransmits = %f, want 10", result.Retransmits)
	}
}

// TestRunWithInvalidBitrate tests the Run function with an invalid bitrate.
func TestRunWithInvalidBitrate(t *testing.T) {
	logger := setupTest(t)

	cfg := Config{
		Target:      "example.com",
		Port:        5201,
		Period:      5 * time.Second,
		Timeout:     10 * time.Second,
		ReverseMode: false,
		Bitrate:     "invalid",
		Logger:      logger,
	}

	result := Run(t.Context(), cfg)

	// Verify the result
	if result.Success {
		t.Error("Run() returned success, expected failure")
	}

	if result.Error == nil {
		t.Error("Run() did not return an error for invalid bitrate")
	}
}

// TestRunWithCommandFailure tests the Run function when the command fails.
func TestRunWithCommandFailure(t *testing.T) {
	logger := setupTest(t)

	// Create a mock execCommand function
	defer ResetExecCommand()

	// Mock both execCommand and execCommandContext functions to return a failing command
	execCommand = func(command string, args ...string) *exec.Cmd {
		// Create a command that will fail
		cmd := exec.Command("false")
		return cmd
	}

	execCommandContext = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		// Use the same mock for context-aware commands
		cmd := exec.Command("false")
		return cmd
	}

	// Run the iperf test
	cfg := Config{
		Target:      "example.com",
		Port:        5201,
		Period:      5 * time.Second,
		Timeout:     10 * time.Second,
		ReverseMode: false,
		Bitrate:     "",
		Logger:      logger,
	}

	result := Run(t.Context(), cfg)

	// Verify the result
	if result.Success {
		t.Error("Run() returned success, expected failure")
	}

	if result.Error == nil {
		t.Error("Run() did not return an error for command failure")
	}
}

// TestRunWithInvalidJSON tests the Run function with invalid JSON output.
func TestRunWithInvalidJSON(t *testing.T) {
	logger := setupTest(t)

	// Create a mock execCommand function
	defer ResetExecCommand()

	// Mock both execCommand and execCommandContext functions to return invalid JSON
	execCommand = func(command string, args ...string) *exec.Cmd {
		// Return invalid JSON
		cmd := exec.Command("echo", "invalid json")
		return cmd
	}

	execCommandContext = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		// Use the same mock for context-aware commands
		cmd := exec.Command("echo", "invalid json")
		return cmd
	}

	// Run the iperf test
	cfg := Config{
		Target:      "example.com",
		Port:        5201,
		Period:      5 * time.Second,
		Timeout:     10 * time.Second,
		ReverseMode: false,
		Bitrate:     "",
		Logger:      logger,
	}

	result := Run(t.Context(), cfg)

	// Verify the result
	if result.Success {
		t.Error("Run() returned success, expected failure")
	}

	if result.Error == nil {
		t.Error("Run() did not return an error for invalid JSON")
	}
}

// TestRunWithTimeout tests the Run function with a timeout.
func TestRunWithTimeout(t *testing.T) {
	logger := setupTest(t)

	// Create a mock execCommand function
	defer ResetExecCommand()

	// Mock both execCommand and execCommandContext functions to sleep longer than the timeout
	execCommand = func(command string, args ...string) *exec.Cmd {
		// Create a command that will sleep longer than the timeout
		cmd := exec.Command("sleep", "2")
		return cmd
	}

	execCommandContext = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		// Use the same mock for context-aware commands
		cmd := exec.Command("sleep", "2")
		return cmd
	}

	// Create a context with a short timeout
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	// Run the iperf test
	cfg := Config{
		Target:      "example.com",
		Port:        5201,
		Period:      5 * time.Second,
		Timeout:     10 * time.Second,
		ReverseMode: false,
		Bitrate:     "",
		Logger:      logger,
	}

	result := Run(ctx, cfg)

	// Verify the result
	if result.Success {
		t.Error("Run() returned success, expected failure due to timeout")
	}

	if result.Error == nil {
		t.Error("Run() did not return an error for timeout")
	}
}

// TestHelperProcess is not a real test, it's used as a helper for TestRunWithMockCommand.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	// Check if we should fail the command
	if os.Getenv("IPERF_FAIL") == "1" {
		os.Exit(1)
	}

	// Check if we should sleep
	if sleepStr := os.Getenv("IPERF_SLEEP"); sleepStr != "" {
		sleepSec, _ := time.ParseDuration(sleepStr + "s")
		time.Sleep(sleepSec)
	}

	// Output the mock iperf result
	if output := os.Getenv("IPERF_OUTPUT"); output != "" {
		// Validate that the output is valid JSON if it's not "invalid json"
		if output != "invalid json" {
			var result map[string]interface{}
			if err := json.Unmarshal([]byte(output), &result); err != nil {
				os.Stderr.WriteString("Invalid JSON in IPERF_OUTPUT: " + err.Error())
				os.Exit(1)
			}
		}

		os.Stdout.WriteString(output)
	}

	os.Exit(0)
}
