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

// Package validation provides common validation utilities and types
package validation

import (
	"fmt"
)

// ValidationError represents a validation error for a specific field
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// NewValidationError creates a new ValidationError
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

// MultiError holds multiple validation errors
type MultiError struct {
	Errors []*ValidationError
}

func (m *MultiError) Error() string {
	if len(m.Errors) == 1 {
		return m.Errors[0].Error()
	}

	msg := "multiple validation errors:"
	for _, err := range m.Errors {
		msg += fmt.Sprintf("\n- %s", err.Error())
	}
	return msg
}

// AddError adds a validation error to the multi-error
func (m *MultiError) AddError(field, message string) {
	m.Errors = append(m.Errors, NewValidationError(field, message))
}

// HasErrors returns true if there are any validation errors
func (m *MultiError) HasErrors() bool {
	return len(m.Errors) > 0
}

// Validator interface for types that can validate themselves
type Validator interface {
	Validate() error
}
