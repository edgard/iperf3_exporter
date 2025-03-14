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

// ProbeConfig represents the configuration for a single probe.
type ProbeConfig struct {
	Target      string
	Port        int
	Period      time.Duration
	Timeout     time.Duration
	ReverseMode bool
	UDPMode     bool
	Bitrate     string
}

// Collector implements the prometheus.Collector interface for iperf3 metrics.
type Collector struct {
	target  string
	port    int
	period  time.Duration
	timeout time.Duration
	mutex   sync.RWMutex
	reverse bool
	udpMode bool
	bitrate string
	logger  *slog.Logger
	runner  iperf.Runner

	// Metrics
	up              *prometheus.Desc
	sentSeconds     *prometheus.Desc
	sentBytes       *prometheus.Desc
	receivedSeconds *prometheus.Desc
	receivedBytes   *prometheus.Desc
	// TCP-specific metrics
	retransmits *prometheus.Desc
	// UDP-specific metrics
	sentPackets     *prometheus.Desc
	sentJitter      *prometheus.Desc
	sentLostPackets *prometheus.Desc
	sentLostPercent *prometheus.Desc
	recvPackets     *prometheus.Desc
	recvJitter      *prometheus.Desc
	recvLostPackets *prometheus.Desc
	recvLostPercent *prometheus.Desc
}

// NewCollector creates a new Collector for iperf3 metrics.
func NewCollector(config ProbeConfig, logger *slog.Logger) *Collector {
	return NewCollectorWithRunner(config, logger, iperf.NewRunner(logger))
}

// NewCollectorWithRunner creates a new Collector for iperf3 metrics with a custom runner.
func NewCollectorWithRunner(config ProbeConfig, logger *slog.Logger, runner iperf.Runner) *Collector {
	// Common labels for all metrics
	labels := []string{"target", "port"}

	return &Collector{
		target:  config.Target,
		port:    config.Port,
		period:  config.Period,
		timeout: config.Timeout,
		reverse: config.ReverseMode,
		udpMode: config.UDPMode,
		bitrate: config.Bitrate,
		logger:  logger,
		runner:  runner,

		// Define metrics with labels
		up: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "up"),
			"Was the last iperf3 probe successful (1 for success, 0 for failure).",
			labels, nil,
		),
		sentSeconds: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "sent_seconds"),
			"Total seconds spent sending packets.",
			labels, nil,
		),
		sentBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "sent_bytes"),
			"Total sent bytes for the last test run.",
			labels, nil,
		),
		receivedSeconds: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "received_seconds"),
			"Total seconds spent receiving packets.",
			labels, nil,
		),
		receivedBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "received_bytes"),
			"Total received bytes for the last test run.",
			labels, nil,
		),
		// TCP-specific metrics
		retransmits: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "retransmits"),
			"Total retransmits for the last test run.",
			labels, nil,
		),
		// UDP-specific metrics
		sentPackets: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "sent_packets"),
			"Total sent packets for the last UDP test run.",
			labels, nil,
		),
		sentJitter: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "sent_jitter_ms"),
			"Jitter in milliseconds for sent packets in UDP mode.",
			labels, nil,
		),
		sentLostPackets: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "sent_lost_packets"),
			"Total lost packets from the sender in the last UDP test run.",
			labels, nil,
		),
		sentLostPercent: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "sent_lost_percent"),
			"Percentage of packets lost from the sender in the last UDP test run.",
			labels, nil,
		),
		recvPackets: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "received_packets"),
			"Total received packets for the last UDP test run.",
			labels, nil,
		),
		recvJitter: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "received_jitter_ms"),
			"Jitter in milliseconds for received packets in UDP mode.",
			labels, nil,
		),
		recvLostPackets: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "received_lost_packets"),
			"Total lost packets at the receiver in the last UDP test run.",
			labels, nil,
		),
		recvLostPercent: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "received_lost_percent"),
			"Percentage of packets lost at the receiver in the last UDP test run.",
			labels, nil,
		),
	}
}

// Describe implements the prometheus.Collector interface.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.up
	ch <- c.sentSeconds
	ch <- c.sentBytes
	ch <- c.receivedSeconds
	ch <- c.receivedBytes

	// TCP-specific metrics
	ch <- c.retransmits

	// UDP-specific metrics
	ch <- c.sentPackets
	ch <- c.sentJitter
	ch <- c.sentLostPackets
	ch <- c.sentLostPercent
	ch <- c.recvPackets
	ch <- c.recvJitter
	ch <- c.recvLostPackets
	ch <- c.recvLostPercent
}

// Collect implements the prometheus.Collector interface.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.mutex.Lock() // To protect metrics from concurrent collects.
	defer c.mutex.Unlock()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	// Run iperf3 test
	result := c.runner.Run(ctx, iperf.Config{
		Target:      c.target,
		Port:        c.port,
		Period:      c.period,
		Timeout:     c.timeout,
		ReverseMode: c.reverse,
		UDPMode:     c.udpMode,
		Bitrate:     c.bitrate,
		Logger:      c.logger,
	})

	// Common label values for all metrics
	labelValues := []string{c.target, strconv.Itoa(c.port)}

	// Set metrics based on result
	if result.Success {
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 1, labelValues...)
		ch <- prometheus.MustNewConstMetric(c.sentSeconds, prometheus.GaugeValue, result.SentSeconds, labelValues...)
		ch <- prometheus.MustNewConstMetric(c.sentBytes, prometheus.GaugeValue, result.SentBytes, labelValues...)
		ch <- prometheus.MustNewConstMetric(c.receivedSeconds, prometheus.GaugeValue, result.ReceivedSeconds, labelValues...)
		ch <- prometheus.MustNewConstMetric(c.receivedBytes, prometheus.GaugeValue, result.ReceivedBytes, labelValues...)

		// Retransmits is only relevant in TCP mode
		if !result.UDPMode {
			ch <- prometheus.MustNewConstMetric(c.retransmits, prometheus.GaugeValue, result.Retransmits, labelValues...)
		}

		// Include UDP-specific metrics when in UDP mode
		if result.UDPMode {
			ch <- prometheus.MustNewConstMetric(c.sentPackets, prometheus.GaugeValue, result.SentPackets, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.sentJitter, prometheus.GaugeValue, result.SentJitter, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.sentLostPackets, prometheus.GaugeValue, result.SentLostPackets, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.sentLostPercent, prometheus.GaugeValue, result.SentLostPercent, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.recvPackets, prometheus.GaugeValue, result.ReceivedPackets, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.recvJitter, prometheus.GaugeValue, result.ReceivedJitter, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.recvLostPackets, prometheus.GaugeValue, result.ReceivedLostPackets, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.recvLostPercent, prometheus.GaugeValue, result.ReceivedLostPercent, labelValues...)
		}
	} else {
		// Return common metrics with 0 values when iperf3 fails
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 0, labelValues...)
		ch <- prometheus.MustNewConstMetric(c.sentSeconds, prometheus.GaugeValue, 0, labelValues...)
		ch <- prometheus.MustNewConstMetric(c.sentBytes, prometheus.GaugeValue, 0, labelValues...)
		ch <- prometheus.MustNewConstMetric(c.receivedSeconds, prometheus.GaugeValue, 0, labelValues...)
		ch <- prometheus.MustNewConstMetric(c.receivedBytes, prometheus.GaugeValue, 0, labelValues...)

		// Only include mode-specific metrics for the active mode
		if !result.UDPMode {
			// TCP-specific metrics on failure
			ch <- prometheus.MustNewConstMetric(c.retransmits, prometheus.GaugeValue, 0, labelValues...)
		} else {
			// UDP-specific metrics on failure
			ch <- prometheus.MustNewConstMetric(c.sentPackets, prometheus.GaugeValue, 0, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.sentJitter, prometheus.GaugeValue, 0, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.sentLostPackets, prometheus.GaugeValue, 0, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.sentLostPercent, prometheus.GaugeValue, 0, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.recvPackets, prometheus.GaugeValue, 0, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.recvJitter, prometheus.GaugeValue, 0, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.recvLostPackets, prometheus.GaugeValue, 0, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.recvLostPercent, prometheus.GaugeValue, 0, labelValues...)
		}

		IperfErrors.Inc()
	}
}
