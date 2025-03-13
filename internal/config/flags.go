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
	"log/slog"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/common/version"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
)

// ParseFlags parses the command line flags and returns a Config.
func ParseFlags() *Config {
	cfg := &Config{
		ListenAddress: DefaultListenAddress,
		MetricsPath:   DefaultMetricsPath,
		ProbePath:     DefaultProbePath,
		Timeout:       DefaultTimeout,
	}

	// Define command-line flags
	kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").
		Default(cfg.MetricsPath).StringVar(&cfg.MetricsPath)

	kingpin.Flag("web.probe-path", "Path under which to expose the probe endpoint.").
		Default(cfg.ProbePath).StringVar(&cfg.ProbePath)

	kingpin.Flag("iperf3.timeout", "iperf3 run timeout.").
		Default(cfg.Timeout.String()).DurationVar(&cfg.Timeout)

	// Set up logging flags
	logLevel := kingpin.Flag("log.level", "Only log messages with the given severity or above. One of: [debug, info, warn, error]").
		Default("info").String()

	logFormat := kingpin.Flag("log.format", "Output format of log messages. One of: [logfmt, json]").
		Default("logfmt").String()

	// Version information
	kingpin.Version(version.Print("iperf3_exporter"))
	kingpin.HelpFlag.Short('h')

	// Parse flags
	kingpin.Parse()

	// Configure components
	cfg.WebConfig = webflag.AddFlags(kingpin.CommandLine, DefaultListenAddress)
	cfg.Logger = setupLogger(*logLevel, *logFormat)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		cfg.Logger.Error("Invalid configuration", "err", err)
		os.Exit(1)
	}

	return cfg
}

// setupLogger configures and returns a slog.Logger based on command-line flags.
func setupLogger(level, format string) *slog.Logger {
	var logLevelSlog slog.Level
	switch level {
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
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevelSlog})
	} else {
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevelSlog})
	}

	return slog.New(handler)
}
