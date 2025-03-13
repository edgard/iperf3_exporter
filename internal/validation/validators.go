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

package validation

import (
	"fmt"
	"regexp"
	"time"
)

// ValidateDuration validates a duration is within acceptable bounds
func ValidateDuration(field string, d time.Duration, min, max time.Duration) error {
	if d < min {
		return NewValidationError(field, fmt.Sprintf("must be at least %v", min))
	}
	if max > 0 && d > max {
		return NewValidationError(field, fmt.Sprintf("must not exceed %v", max))
	}
	return nil
}

// ValidateBitrate validates the bitrate format
// Format: #[KMG][/#], where # is a number
func ValidateBitrate(bitrate string) error {
	if bitrate == "" {
		return nil // Empty is valid (uses default)
	}

	// Regular expression for valid bitrate format
	// Examples: "1M", "100K", "1G", "1M/100"
	pattern := `^\d+[KMG](/\d+)?$`
	matched, err := regexp.MatchString(pattern, bitrate)
	if err != nil {
		return NewValidationError("bitrate", "internal validation error")
	}
	if !matched {
		return NewValidationError("bitrate", "must be in format #[KMG][/#] (e.g., '1M', '100K/10')")
	}

	return nil
}

// ValidatePort validates a port number
func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return NewValidationError("port", "must be between 1 and 65535")
	}
	return nil
}
