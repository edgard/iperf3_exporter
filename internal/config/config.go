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

// Package config provides configuration handling for the iperf3_exporter.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
)

// Config represents the configuration for the iperf3_exporter.
type Config struct {
	ListenAddress string
	MetricsPath   string
	ProbePath     string
	Timeout       time.Duration
	Logger        *slog.Logger
	WebConfig     *web.FlagConfig
}

// NewConfig creates a new Config with default values.
func NewConfig() *Config {
	return &Config{
		ListenAddress: ":9579",
		MetricsPath:   "/metrics",
		ProbePath:     "/probe",
		Timeout:       30 * time.Second,
	}
}

// ParseFlags parses the command line flags and returns a Config.
func ParseFlags() *Config {
	cfg := NewConfig()

	// Define command-line flags (note: web.listen-address is handled by exporter-toolkit)

	kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").
		Default(cfg.MetricsPath).StringVar(&cfg.MetricsPath)

	kingpin.Flag("web.probe-path", "Path under which to expose the probe endpoint.").
		Default(cfg.ProbePath).StringVar(&cfg.ProbePath)

	kingpin.Flag("iperf3.timeout", "iperf3 run timeout.").
		Default(cfg.Timeout.String()).DurationVar(&cfg.Timeout)

	// Add web configuration flags (TLS, basic auth, etc.)
	cfg.WebConfig = webflag.AddFlags(kingpin.CommandLine, cfg.ListenAddress)

	// Set up logging
	logLevel := kingpin.Flag("log.level", "Only log messages with the given severity or above. One of: [debug, info, warn, error]").
		Default("info").String()

	logFormat := kingpin.Flag("log.format", "Output format of log messages. One of: [logfmt, json]").
		Default("logfmt").String()

	// Version information
	kingpin.Version(version.Print("iperf3_exporter"))
	kingpin.HelpFlag.Short('h')

	// Parse flags
	kingpin.Parse()

	// Initialize logger
	var logLevelSlog slog.Level

	switch *logLevel {
	case "debug":
		logLevelSlog = slog.LevelDebug
	case "info":
		logLevelSlog = slog.LevelInfo
	case "warn":
		logLevelSlog = slog.LevelWarn
	case "error":
		logLevelSlog = slog.LevelError
	default:
		logLevelSlog = slog.LevelInfo
	}

	var handler slog.Handler
	if *logFormat == "json" {
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevelSlog})
	} else {
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevelSlog})
	}

	cfg.Logger = slog.New(handler)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		cfg.Logger.Error("Invalid configuration", "err", err)
		os.Exit(1)
	}

	return cfg
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	// Validate basic configuration
	if c.MetricsPath == "" {
		return fmt.Errorf("metrics path cannot be empty")
	}

	if c.ProbePath == "" {
		return fmt.Errorf("probe path cannot be empty")
	}

	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be greater than 0")
	}

	if c.Logger == nil {
		return fmt.Errorf("logger cannot be nil")
	}

	// Validate web configuration
	if c.WebConfig == nil {
		return fmt.Errorf("web configuration cannot be nil")
	}

	// Note: Additional web config validation is handled by web.ListenAndServe
	// which checks for listen addresses and validates TLS config if provided

	return nil
}
