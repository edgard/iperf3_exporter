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

// Package collector provides the Prometheus collector for iperf3 metrics.
package collector

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/edgard/iperf3_exporter/internal/iperf"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace = "iperf3"
)

// ProbeConfig represents the configuration for a single probe.
type ProbeConfig struct {
	Target      string
	Port        int
	Period      time.Duration
	Timeout     time.Duration
	ReverseMode bool
	Bitrate     string
}

// Collector implements the prometheus.Collector interface for iperf3 metrics.
type Collector struct {
	config ProbeConfig
	logger *slog.Logger
	runner iperf.Runner
	mutex  sync.RWMutex
	descs  map[string]*prometheus.Desc
}

// Metrics about the iperf3 exporter itself.
var (
	IperfDuration = prometheus.NewSummary(
		prometheus.SummaryOpts{
			Name: prometheus.BuildFQName(namespace, "exporter", "duration_seconds"),
			Help: "Duration of collections by the iperf3 exporter.",
		},
	)
	IperfErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: prometheus.BuildFQName(namespace, "exporter", "errors_total"),
			Help: "Errors raised by the iperf3 exporter.",
		},
	)
)

// NewCollector creates a new Collector for iperf3 metrics.
func NewCollector(config ProbeConfig, logger *slog.Logger) *Collector {
	return NewCollectorWithRunner(config, logger, iperf.NewRunner(logger))
}

// NewCollectorWithRunner creates a new Collector for iperf3 metrics with a custom runner.
func NewCollectorWithRunner(config ProbeConfig, logger *slog.Logger, runner iperf.Runner) *Collector {
	labels := []string{"target", "port"}

	// Create metric descriptors map
	descs := map[string]*prometheus.Desc{
		"up": prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "up"),
			"Was the last iperf3 probe successful (1 for success, 0 for failure).",
			labels, nil,
		),
		"sent_seconds": prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "sent_seconds"),
			"Total seconds spent sending packets.",
			labels, nil,
		),
		"sent_bytes": prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "sent_bytes"),
			"Total sent bytes for the last test run.",
			labels, nil,
		),
		"received_seconds": prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "received_seconds"),
			"Total seconds spent receiving packets.",
			labels, nil,
		),
		"received_bytes": prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "received_bytes"),
			"Total received bytes for the last test run.",
			labels, nil,
		),
		"retransmits": prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "retransmits"),
			"Total retransmits for the last test run.",
			labels, nil,
		),
	}

	return &Collector{
		config: config,
		logger: logger,
		runner: runner,
		descs:  descs,
	}
}

// Describe implements the prometheus.Collector interface.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descs {
		ch <- desc
	}
}

// Collect implements the prometheus.Collector interface.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	// Lock the mutex for the entire operation to prevent concurrent iperf3 executions
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), c.config.Timeout)
	defer cancel()

	// Run test
	result := c.runner.Run(ctx, iperf.Config{
		Target:      c.config.Target,
		Port:        c.config.Port,
		Period:      c.config.Period,
		Timeout:     c.config.Timeout,
		ReverseMode: c.config.ReverseMode,
		Bitrate:     c.config.Bitrate,
		Logger:      c.logger,
	})

	// Collect metrics
	labelValues := []string{c.config.Target, strconv.Itoa(c.config.Port)}

	// Set up metric
	upValue := 0.0
	if result.Success {
		upValue = 1.0
	} else {
		IperfErrors.Inc()
	}
	ch <- prometheus.MustNewConstMetric(c.descs["up"], prometheus.GaugeValue, upValue, labelValues...)

	// Set other metrics
	for name, desc := range c.descs {
		if name == "up" {
			continue // Already handled
		}

		val := 0.0
		if result.Success && result.Metrics != nil {
			if metricVal, exists := result.Metrics[name]; exists {
				val = metricVal
			}
		}
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, val, labelValues...)
	}
}
