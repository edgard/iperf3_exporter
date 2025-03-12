# Empty builder stage (used by goreleaser)
FROM scratch AS builder-base
# Create an empty directory structure
WORKDIR /go/bin
# This is just a placeholder and won't actually be used by goreleaser

# Real builder stage (only used by local target)
FROM golang:1.24-alpine AS builder-local
WORKDIR /go/src/github.com/edgard/iperf3_exporter
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 go build -o /go/bin/iperf3_exporter ./cmd/iperf3_exporter

# Base stage with common setup
FROM alpine:3.21 AS base
# Install iperf3 and wget for health check
RUN apk add --no-cache iperf3 wget
# Add non-root user
RUN adduser -D -u 10001 iperf3_exporter
WORKDIR /app
# Expose metrics port
EXPOSE 9579
# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget -q -O- http://localhost:9579/health || exit 1

# GoReleaser target (expects pre-built binary)
FROM base AS goreleaser
# GoReleaser will copy the pre-built binary here
COPY iperf3_exporter /app/iperf3_exporter
USER iperf3_exporter
ENTRYPOINT ["/app/iperf3_exporter"]

# Local development target (uses binary from builder-local)
# This is the default target when no --target flag is provided
FROM base AS local
COPY --from=builder-local /go/bin/iperf3_exporter /app/iperf3_exporter
USER iperf3_exporter
ENTRYPOINT ["/app/iperf3_exporter"]
