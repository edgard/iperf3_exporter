// Package server provides the HTTP server for the iperf3 exporter.
package server

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/edgard/iperf3_exporter/internal/validation"
)

// DefaultValues holds default values for probe request parameters
var DefaultValues = struct {
	Port       int
	Period     time.Duration
	MinTimeout time.Duration
	MaxTimeout time.Duration
	MinPeriod  time.Duration
}{
	Port:       5201,
	Period:     5 * time.Second,
	MinTimeout: 1 * time.Second,
	MaxTimeout: 300 * time.Second, // 5 minutes max timeout
	MinPeriod:  100 * time.Millisecond,
}

// ProbeRequest represents a validated probe request with all parameters
type ProbeRequest struct {
	Target      string
	Port        int
	Period      time.Duration
	Timeout     time.Duration
	ReverseMode bool
	Bitrate     string
}

// ParseProbeRequest parses and validates an HTTP request into a ProbeRequest
func ParseProbeRequest(r *http.Request) (*ProbeRequest, error) {
	query := r.URL.Query()
	req := &ProbeRequest{}

	// Initialize multi-error to collect all validation errors
	merr := &validation.MultiError{}

	// Required: Target parameter
	req.Target = query.Get("target")
	if req.Target == "" {
		merr.AddError("target", "must be specified")
	}

	// Optional: Port parameter (with default)
	if port := query.Get("port"); port != "" {
		var err error
		req.Port, err = strconv.Atoi(port)
		if err != nil {
			merr.AddError("port", fmt.Sprintf("must be an integer, got '%s'", port))
		} else if err := validation.ValidatePort(req.Port); err != nil {
			merr.Errors = append(merr.Errors, err.(*validation.ValidationError))
		}
	} else {
		req.Port = DefaultValues.Port
	}

	// Optional: Period parameter (with default)
	if period := query.Get("period"); period != "" {
		var err error
		req.Period, err = time.ParseDuration(period)
		if err != nil {
			merr.AddError("period", fmt.Sprintf("invalid duration format: %s", err))
		}
	} else {
		req.Period = DefaultValues.Period
	}

	// Optional: Reverse mode parameter
	if reverse := query.Get("reverse_mode"); reverse != "" {
		var err error
		req.ReverseMode, err = strconv.ParseBool(reverse)
		if err != nil {
			merr.AddError("reverse_mode", "must be true or false")
		}
	}

	// Optional: Bitrate parameter
	req.Bitrate = query.Get("bitrate")
	if req.Bitrate != "" {
		if err := validation.ValidateBitrate(req.Bitrate); err != nil {
			merr.Errors = append(merr.Errors, err.(*validation.ValidationError))
		}
	}

	// Get timeout from Prometheus header or use default
	timeoutSeconds := DefaultValues.MaxTimeout.Seconds()
	if v := r.Header.Get("X-Prometheus-Scrape-Timeout-Seconds"); v != "" {
		var err error
		timeoutSeconds, err = strconv.ParseFloat(v, 64)
		if err != nil {
			merr.AddError("timeout", fmt.Sprintf("invalid timeout value in header: %s", err))
		}
	}
	req.Timeout = time.Duration(timeoutSeconds * float64(time.Second))

	if merr.HasErrors() {
		return nil, merr
	}

	// Post-parse validation of the complete request
	return req, req.Validate()
}

// Validate performs validation on the complete ProbeRequest
func (r *ProbeRequest) Validate() error {
	merr := &validation.MultiError{}

	// Validate period is within bounds
	if err := validation.ValidateDuration("period", r.Period, DefaultValues.MinPeriod, r.Timeout); err != nil {
		merr.Errors = append(merr.Errors, err.(*validation.ValidationError))
	}

	// Validate timeout is within bounds
	if err := validation.ValidateDuration("timeout", r.Timeout, DefaultValues.MinTimeout, DefaultValues.MaxTimeout); err != nil {
		merr.Errors = append(merr.Errors, err.(*validation.ValidationError))
	}

	// Ensure period is less than timeout
	if r.Period >= r.Timeout {
		r.Period = time.Duration(float64(r.Timeout) * 0.9) // Set period to 90% of timeout
		// We don't return an error here as we've automatically adjusted the value
	}

	if merr.HasErrors() {
		return merr
	}

	return nil
}
