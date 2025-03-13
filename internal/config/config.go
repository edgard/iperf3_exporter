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
	"errors"
	"log/slog"
	"time"

	"github.com/prometheus/exporter-toolkit/web"
)

// Default configuration values
const (
	DefaultListenAddress = ":9579"
	DefaultMetricsPath   = "/metrics"
	DefaultProbePath     = "/probe"
	DefaultTimeout       = 30 * time.Second
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

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.MetricsPath == "" {
		return errors.New("metrics path cannot be empty")
	}

	if c.ProbePath == "" {
		return errors.New("probe path cannot be empty")
	}

	if c.Timeout <= 0 {
		return errors.New("timeout must be greater than 0")
	}

	if c.Logger == nil {
		return errors.New("logger cannot be nil")
	}

	if c.WebConfig == nil {
		return errors.New("web configuration cannot be nil")
	}

	return nil
}
