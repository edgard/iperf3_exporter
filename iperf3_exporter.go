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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
)

const (
	namespace = "iperf3"
)

var (
	listenAddress = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9579").String()
	metricsPath   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
	timeout       = kingpin.Flag("iperf3.timeout", "iperf3 run timeout.").Default("30s").Duration()

	// Metrics about the iperf3 exporter itself.
	iperfDuration = prometheus.NewSummary(prometheus.SummaryOpts{Name: prometheus.BuildFQName(namespace, "exporter", "duration_seconds"), Help: "Duration of collections by the iperf3 exporter."})
	iperfErrors   = prometheus.NewCounter(prometheus.CounterOpts{Name: prometheus.BuildFQName(namespace, "exporter", "errors_total"), Help: "Errors raised by the iperf3 exporter."})

	logger log.Logger
)

// iperfResult collects the partial result from the iperf3 run.
type iperfResult struct {
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

// Exporter collects iperf3 stats from the given address and exports them using
// the prometheus metrics package.
type Exporter struct {
	target  string
	port    int
	period  time.Duration
	timeout time.Duration
	mutex   sync.RWMutex
	reverse bool

	success               *prometheus.Desc
	sentSeconds           *prometheus.Desc
	sentBytes             *prometheus.Desc
	sentBitsPerSecond     *prometheus.Desc
	receivedSeconds       *prometheus.Desc
	receivedBytes         *prometheus.Desc
	receivedBitsPerSecond *prometheus.Desc
	retransmits           *prometheus.Desc
}

// NewExporter returns an initialized Exporter.
func NewExporter(target string, port int, period time.Duration, timeout time.Duration, reverse bool) *Exporter {
	return &Exporter{
		target:                target,
		port:                  port,
		period:                period,
		timeout:               timeout,
		reverse:               reverse,
		success:               prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "success"), "Was the last iperf3 probe successful.", nil, nil),
		sentSeconds:           prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "sent_seconds"), "Total seconds spent sending packets.", nil, nil),
		sentBytes:             prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "sent_bytes"), "Total sent bytes.", nil, nil),
		sentBitsPerSecond:     prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "sent_bps"), "Bits per second on sending packets.", nil, nil),
		receivedSeconds:       prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "received_seconds"), "Total seconds spent receiving packets.", nil, nil),
		receivedBytes:         prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "received_bytes"), "Total received bytes.", nil, nil),
		receivedBitsPerSecond: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "received_bps"), "Bits per second on receiving packets.", nil, nil),
		retransmits:           prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "retransmits"), "Total retransmits", nil, nil),
	}
}

// Describe describes all the metrics exported by the iperf3 exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.success
	ch <- e.sentSeconds
	ch <- e.sentBytes
	ch <- e.sentBitsPerSecond
	ch <- e.receivedSeconds
	ch <- e.receivedBytes
	ch <- e.receivedBitsPerSecond
	ch <- e.retransmits
}

// Collect probes the configured iperf3 server and delivers them as Prometheus
// metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock() // To protect metrics from concurrent collects.
	defer e.mutex.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	var iperfArgs []string
	iperfArgs = []string{"-J", "-t", strconv.FormatFloat(e.period.Seconds(), 'f', 0, 64), "-c", e.target, "-p", strconv.Itoa(e.port)}
	if e.reverse {
		iperfArgs = append(iperfArgs, "-R")
	}
	out, err := exec.CommandContext(ctx, iperfCmd, iperfArgs...).Output()
	if err != nil {
		ch <- prometheus.MustNewConstMetric(e.success, prometheus.GaugeValue, 0)

		iperfErrors.Inc()
		level.Error(logger).Log("msg", "Failed to run iperf3", "err", err)

		return
	}

	stats := iperfResult{}
	if err := json.Unmarshal(out, &stats); err != nil {
		ch <- prometheus.MustNewConstMetric(e.success, prometheus.GaugeValue, 0)

		iperfErrors.Inc()
		level.Error(logger).Log("msg", "Failed to parse iperf3 result", "err", err)

		return
	}

	ch <- prometheus.MustNewConstMetric(e.success, prometheus.GaugeValue, 1)
	ch <- prometheus.MustNewConstMetric(e.sentSeconds, prometheus.GaugeValue, stats.End.SumSent.Seconds)
	ch <- prometheus.MustNewConstMetric(e.sentBytes, prometheus.GaugeValue, stats.End.SumSent.Bytes)
	ch <- prometheus.MustNewConstMetric(e.sentBitsPerSecond, prometheus.GaugeValue, stats.End.SumSent.BitsPerSecond)
	ch <- prometheus.MustNewConstMetric(e.receivedSeconds, prometheus.GaugeValue, stats.End.SumReceived.Seconds)
	ch <- prometheus.MustNewConstMetric(e.receivedBytes, prometheus.GaugeValue, stats.End.SumReceived.Bytes)
	ch <- prometheus.MustNewConstMetric(e.receivedBitsPerSecond, prometheus.GaugeValue, stats.End.SumReceived.BitsPerSecond)
	ch <- prometheus.MustNewConstMetric(e.retransmits, prometheus.GaugeValue, stats.End.SumSent.Retransmits)
}

func handler(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("target")
	if target == "" {
		http.Error(w, "'target' parameter must be specified", http.StatusBadRequest)
		iperfErrors.Inc()

		return
	}

	var targetPort int

	port := r.URL.Query().Get("port")
	if port != "" {
		var err error

		targetPort, err = strconv.Atoi(port)
		if err != nil {
			http.Error(w, fmt.Sprintf("'port' parameter must be an integer: %s", err), http.StatusBadRequest)
			iperfErrors.Inc()

			return
		}
	}

	if targetPort == 0 {
		targetPort = 5201
	}

	var reverseMode bool
	reverse_mode := r.URL.Query().Get("reverse_mode")
	if reverse_mode != "" {
		var err error
		reverseMode, err = strconv.ParseBool(reverse_mode)
		if err != nil {
			http.Error(w, fmt.Sprintf("'reverse_mode' parameter must be true or false (boolean): %s", err), http.StatusBadRequest)
			iperfErrors.Inc()

			return
		}
	} else {
		reverseMode = false
	}

	var runPeriod time.Duration

	period := r.URL.Query().Get("period")
	if period != "" {
		var err error

		runPeriod, err = time.ParseDuration(period)
		if err != nil {
			http.Error(w, fmt.Sprintf("'period' parameter must be a duration: %s", err), http.StatusBadRequest)
			iperfErrors.Inc()

			return
		}
	}

	if runPeriod.Seconds() == 0 {
		runPeriod = time.Second * 5
	}

	// If a timeout is configured via the Prometheus header, add it to the request.
	var timeoutSeconds float64

	if v := r.Header.Get("X-Prometheus-Scrape-Timeout-Seconds"); v != "" {
		var err error

		timeoutSeconds, err = strconv.ParseFloat(v, 64)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse timeout from Prometheus header: %s", err), http.StatusInternalServerError)
			iperfErrors.Inc()

			return
		}
	}

	if timeoutSeconds == 0 {
		if timeout.Seconds() > 0 {
			timeoutSeconds = timeout.Seconds()
		} else {
			timeoutSeconds = 30
		}
	}

	if timeoutSeconds > 30 {
		timeoutSeconds = 30
	}

	runTimeout := time.Duration(timeoutSeconds * float64(time.Second))

	start := time.Now()
	registry := prometheus.NewRegistry()
	exporter := NewExporter(target, targetPort, runPeriod, runTimeout, reverseMode)
	registry.MustRegister(exporter)

	// Delegate http serving to Prometheus client library, which will call collector.Collect.
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)

	duration := time.Since(start).Seconds()
	iperfDuration.Observe(duration)
}

func main() {
	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.Version(version.Print("iperf3_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	logger = promlog.New(promlogConfig)
	level.Info(logger).Log("msg", "Starting iperf3 exporter", "version", version.Info())
	level.Info(logger).Log("msg", "Build context", "context", version.BuildContext())

	prometheus.MustRegister(version.NewCollector("iperf3_exporter"))
	prometheus.MustRegister(iperfDuration)
	prometheus.MustRegister(iperfErrors)

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/probe", handler)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")

		_, err := w.Write([]byte(`<html>
    <head><title>iPerf3 Exporter</title></head>
    <body>
    <h1>iPerf3 Exporter</h1>
    <p><a href="/probe?target=prometheus.io">Probe prometheus.io</a></p>
    <p><a href='` + *metricsPath + `'>Metrics</a></p>
    </html>`))
		if err != nil {
			level.Warn(logger).Log("msg", "Failed to write to HTTP client", "err", err)
		}
	})

	srv := &http.Server{
		Addr:         *listenAddress,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	level.Info(logger).Log("msg", "Listening on", "address", srv.Addr)
	level.Error(logger).Log("msg", "Server stopped", "err", srv.ListenAndServe())
}
