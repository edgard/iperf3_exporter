# iPerf3 Exporter

A Prometheus exporter for iPerf3 network performance metrics.

[![Go Report Card](https://goreportcard.com/badge/github.com/edgard/iperf3_exporter)](https://goreportcard.com/report/github.com/edgard/iperf3_exporter)
[![Docker Pulls](https://img.shields.io/docker/pulls/ghcr.io/edgard/iperf3_exporter.svg)](https://github.com/users/edgard/packages/container/package/iperf3_exporter)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/edgard/iperf3_exporter/blob/master/LICENSE)

## ⚠️ IMPORTANT: Docker Image Name Change

**The Docker image has moved to GitHub Container Registry (ghcr.io) and the name has changed from `iperf3-exporter` to `iperf3_exporter` following GitHub's naming standards. If you were using the old image name, please update your references.**

The iPerf3 exporter allows iPerf3 probing of endpoints for Prometheus monitoring, enabling you to measure network performance metrics like bandwidth, jitter, and packet loss.

## Features

- Measure network bandwidth between hosts
- Monitor network performance over time
- Support for both TCP and UDP tests
- Configurable test parameters (duration, bitrate, etc.)
- TLS support for secure communication
- Basic authentication for access control
- Health and readiness endpoints for monitoring
- Prometheus metrics for exporter itself

## Installation

### From Binaries

Download the most suitable binary for your platform from [the releases tab](https://github.com/edgard/iperf3_exporter/releases).

```bash
# Download (replace VERSION and PLATFORM with appropriate values)
curl -L -o iperf3_exporter https://github.com/edgard/iperf3_exporter/releases/download/VERSION/iperf3_exporter-VERSION.PLATFORM

# Make executable
chmod +x iperf3_exporter

# Run
./iperf3_exporter <flags>
```

*Note: [iperf3](https://iperf.fr/) binary should also be installed and accessible from the path.*

### Using Docker

```bash
docker run --rm -d -p 9579:9579 --name iperf3_exporter ghcr.io/edgard/iperf3_exporter:latest
```

The Docker images are available for multiple architectures (amd64, arm64) and are published to GitHub Container Registry.

### Building from Source

```bash
# Clone repository
git clone https://github.com/edgard/iperf3_exporter.git
cd iperf3_exporter

# Build
go build -o iperf3_exporter ./cmd/iperf3_exporter

# Run
./iperf3_exporter
```

## Usage

### Starting the Exporter

```bash
./iperf3_exporter [flags]
```

### Configuration

iPerf3 exporter is configured via command-line flags:

| Flag | Description | Default |
|------|-------------|---------|
| `--web.listen-address` | Addresses on which to expose metrics and web interface (repeatable) | `:9579` |
| `--web.telemetry-path` | Path under which to expose metrics | `/metrics` |
| `--web.probe-path` | Path under which to expose the probe endpoint | `/probe` |
| `--iperf3.timeout` | iperf3 run timeout | `30s` |
| `--web.config.file` | Path to configuration file that can enable TLS or authentication | |
| `--web.systemd-socket` | Use systemd socket activation listeners instead of port listeners (Linux only) | `false` |
| `--log.level` | Only log messages with the given severity or above | `info` |
| `--log.format` | Output format of log messages | `logfmt` |

#### Web Configuration File

The exporter supports a configuration file for TLS and authentication settings. This file is specified with the `--web.config.file` flag.

Example configuration file:

```yaml
tls_server_config:
  cert_file: server.crt
  key_file: server.key

basic_auth_users:
  username: password
```

For more details on the web configuration file format, see the [exporter-toolkit documentation](https://github.com/prometheus/exporter-toolkit/blob/master/docs/web-configuration.md).

To view all available command-line flags, run:

```bash
./iperf3_exporter -h
```

The timeout of each probe is automatically determined from the `scrape_timeout` in the [Prometheus config](https://prometheus.io/docs/operating/configuration/#configuration-file).
This can be also be limited by the `iperf3.timeout` command-line flag. If neither is specified, it defaults to 30 seconds.

### Probe Parameters

When making requests to the `/probe` endpoint, the following parameters can be used:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `target` | Target host to probe (required) | - |
| `port` | Port that the target iperf3 server is listening on | 5201 |
| `reverse_mode` | Run iperf3 in reverse mode (server sends, client receives) | false |
| `udp_mode` | Run iperf3 in UDP mode instead of TCP | false |
| `bitrate` | Target bitrate in bits/sec (format: #[KMG][/#]). For UDP mode, iperf3 defaults to 1 Mbit/sec if not specified. | - |
| `period` | Duration of the iperf3 test | 5s |

### Checking the Results

Visit [http://localhost:9579](http://localhost:9579) to see the exporter's web interface.

## Prometheus Configuration

The iPerf3 exporter needs to be passed the target as a parameter, this can be done with relabelling.

Example config:
```yml
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
      # Optional: enable UDP mode
      # udp_mode: ['true']
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
```

### Available Metrics

The exporter provides the following metrics:

| Metric | Description | Labels |
|--------|-------------|--------|
| `iperf3_up` | Was the last iperf3 probe successful (1 for success, 0 for failure) | `target`, `port` |
| `iperf3_sent_seconds` | Total seconds spent sending packets | `target`, `port` |
| `iperf3_sent_bytes` | Total sent bytes for the last test run | `target`, `port` |
| `iperf3_received_seconds` | Total seconds spent receiving packets | `target`, `port` |
| `iperf3_received_bytes` | Total received bytes for the last test run | `target`, `port` |
| `iperf3_retransmits` | Total retransmits for the last test run (TCP mode only, omitted in UDP) | `target`, `port` |
| `iperf3_sent_packets` | Total sent packets for the last UDP test run (UDP mode only) | `target`, `port` |
| `iperf3_sent_jitter_ms` | Jitter in milliseconds for sent packets (UDP mode only) | `target`, `port` |
| `iperf3_lost_packets` | Total lost packets for the last UDP test run (UDP mode only) | `target`, `port` |
| `iperf3_lost_percent` | Percentage of packets lost for the last UDP test run (UDP mode only) | `target`, `port` |

Additionally, the exporter provides metrics about itself:

| Metric | Description |
|--------|-------------|
| `iperf3_exporter_duration_seconds` | Duration of collections by the iperf3 exporter |
| `iperf3_exporter_errors_total` | Errors raised by the iperf3 exporter |

### Querying the Bandwidth

You can use the following Prometheus queries to calculate bandwidth in Mbits/sec:

#### Receiver Bandwidth (Download Speed)
```
rate(iperf3_received_bytes{instance="target"}[1m]) * 8 / 1000000
```

#### Sender Bandwidth (Upload Speed)
```
rate(iperf3_sent_bytes{instance="target"}[1m]) * 8 / 1000000
```

These queries use the `rate()` function to calculate the per-second rate from the counter metrics, then convert from bytes to bits (multiply by 8) and from bits to megabits (divide by 1,000,000).

## Contributing

Contributions to the iperf3_exporter are welcome!

This project follows the [Conventional Commits](https://www.conventionalcommits.org/) specification. When contributing, please format your commit messages according to this standard:

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

Examples:
- `feat: add support for UDP tests`
- `fix: correct metric label in collector`
- `docs: update installation instructions`
- `refactor(collector): simplify error handling`

### Development Prerequisites

- Go 1.24 or higher
- iperf3 installed on your system

### Project Structure

```
.
├── cmd/
│   └── iperf3_exporter/     # Main application entry point
├── internal/
│   ├── collector/           # Prometheus collector implementation
│   ├── config/              # Configuration handling
│   ├── iperf/               # iperf3 command execution and result parsing
│   └── server/              # HTTP server implementation
├── tests/
│   └── e2e/                 # End-to-end tests
├── .github/
│   └── workflows/           # GitHub Actions workflows
├── .goreleaser.yml          # GoReleaser configuration
├── Dockerfile               # Multi-arch Docker build configuration
├── go.mod                   # Go module definition
└── README.md                # This file
```

### Building and Testing

The project uses a Makefile to streamline development tasks:

```bash
# Build the binary
make build

# Run all tests
make test

# Complete development workflow (run mod, generate, lint, vet, tests and build)
make all

# Tidy and download dependencies
make mod

# Run linting
make lint

# Run go vet
make vet

# Generate code (if any generators are configured)
make generate

# Build Docker image for local development
make docker

# See all available commands
make help
```

You can also use standard Go commands directly:

```bash
# Build manually
go build -o iperf3_exporter ./cmd/iperf3_exporter

# Run tests
go test ./...
```

## License

This project is released under Apache License 2.0, see [LICENSE](https://github.com/edgard/iperf3_exporter/blob/master/LICENSE).
