package forge

import (
	"fmt"
	"strings"
)

// ValidationErrors represents a collection of validation errors
type ValidationErrors struct {
	errors []error
}

// NewValidationErrors creates a new ValidationErrors instance
func NewValidationErrors() *ValidationErrors {
	return &ValidationErrors{
		errors: make([]error, 0),
	}
}

// Add adds an error to the collection
func (ve *ValidationErrors) Add(err error) {
	if err != nil {
		ve.errors = append(ve.errors, err)
	}
}

// AddErrorf adds a formatted error to the collection
func (ve *ValidationErrors) AddErrorf(format string, args ...interface{}) {
	ve.errors = append(ve.errors, fmt.Errorf(format, args...))
}

// AddAll adds all errors from another ValidationErrors instance
func (ve *ValidationErrors) AddAll(other *ValidationErrors) {
	if other != nil {
		ve.errors = append(ve.errors, other.errors...)
	}
}

// HasErrors returns true if there are any validation errors
func (ve *ValidationErrors) HasErrors() bool {
	return len(ve.errors) > 0
}

// Count returns the number of validation errors
func (ve *ValidationErrors) Count() int {
	return len(ve.errors)
}

// Error implements the error interface
func (ve *ValidationErrors) Error() string {
	if len(ve.errors) == 0 {
		return ""
	}

	if len(ve.errors) == 1 {
		return ve.errors[0].Error()
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("validation failed with %d errors:\n", len(ve.errors)))
	for i, err := range ve.errors {
		b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, err.Error()))
	}
	return b.String()
}

// ErrorOrNil returns the ValidationErrors as an error if there are any errors, otherwise nil
func (ve *ValidationErrors) ErrorOrNil() error {
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// ValidateURI validates that a URI string is not empty and follows basic format
func ValidateURI(uri, context string) error {
	if uri == "" {
		return fmt.Errorf("%s: URI cannot be empty", context)
	}

	// Basic check for protocol separator
	if !strings.Contains(uri, "://") {
		return fmt.Errorf("%s: URI %q must contain protocol separator (://)", context, uri)
	}

	return nil
}

// ValidateRequired validates that a string field is not empty
func ValidateRequired(value, fieldName, context string) error {
	if value == "" {
		return fmt.Errorf("%s: %s is required", context, fieldName)
	}
	return nil
}

// Contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
