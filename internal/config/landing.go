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
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
)

// GetLandingConfig returns the landing page configuration.
func (c *Config) GetLandingConfig() web.LandingConfig {
	config := web.LandingConfig{
		Name:        "iPerf3 Exporter",
		Description: "The iPerf3 exporter allows iPerf3 probing of endpoints for Prometheus monitoring.",
		Version:     version.Info(),
		Links: []web.LandingLinks{
			{
				Address: c.MetricsPath,
				Text:    "Metrics",
			},
		},
		HeaderColor: "#1a73e8",
	}

	// Add probe information
	config.ExtraHTML = `
    <h2>Quick Start</h2>
    <p>To probe a target:</p>
    <pre><a href="/probe?target=example.com">/probe?target=example.com</a></pre>

    <h2>Probe Parameters</h2>
    <table>
        <tr>
            <th>Parameter</th>
            <th>Description</th>
            <th>Default</th>
        </tr>
        <tr>
            <td>target</td>
            <td>Target host to probe (required)</td>
            <td>-</td>
        </tr>
        <tr>
            <td>port</td>
            <td>Port that the target iperf3 server is listening on</td>
            <td>5201</td>
        </tr>
        <tr>
            <td>reverse_mode</td>
            <td>Run iperf3 in reverse mode (server sends, client receives)</td>
            <td>false</td>
        </tr>
        <tr>
            <td>bitrate</td>
            <td>Target bitrate in bits/sec (format: #[KMG][/#])</td>
            <td>-</td>
        </tr>
        <tr>
            <td>period</td>
            <td>Duration of the iperf3 test</td>
            <td>5s</td>
        </tr>
    </table>

    <h2>Prometheus Configuration Example</h2>
    <pre>
scrape_configs:
  - job_name: 'iperf3'
    metrics_path: /probe
    static_configs:
      - targets:
        - foo.server
        - bar.server
    params:
      port: ['5201']
      # Optional: enable reverse mode
      # reverse_mode: ['true']
      # Optional: set bitrate limit
      # bitrate: ['100M']
      # Optional: set test period
      # period: ['10s']
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: 127.0.0.1:9579  # The iPerf3 exporter's real hostname:port.
    </pre>
    `

	// Add custom CSS
	config.ExtraCSS = `
    table {
        border-collapse: collapse;
        width: 100%;
    }
    th, td {
        text-align: left;
        padding: 8px;
        border-bottom: 1px solid #ddd;
    }
    th {
        background-color: #f2f2f2;
    }
    pre {
        background-color: #f5f5f5;
        padding: 10px;
        border-radius: 5px;
        overflow-x: auto;
    }
    `

	return config
}
