// Package validator provides request parameter validation utilities.
package validator

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

// ValidationError represents a validation failure.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error implements the error interface.
func (ve *ValidationError) Error() string {
	return fmt.Sprintf("validation error: field '%s' - %s", ve.Field, ve.Message)
}

// ValidationErrors is a collection of validation errors.
type ValidationErrors []ValidationError

// Error implements the error interface for multiple errors.
func (ves ValidationErrors) Error() string {
	msgs := make([]string, len(ves))
	for i, ve := range ves {
		msgs[i] = ve.Error()
	}
	return strings.Join(msgs, "; ")
}

// IsValidationError checks if an error is a ValidationError.
func IsValidationError(err error) bool {
	_, ok := err.(ValidationErrors)
	if ok {
		return true
	}
	_, ok2 := err.(*ValidationError)
	return ok2
}

// Validator holds validation rules for a request.
type Validator struct {
	rules []rule
}

// rule represents a single validation rule.
type rule struct {
	field   string
	value   interface{}
	checks  []checkFunc
}

// checkFunc is a validation function.
type checkFunc func(field string, value interface{}) *ValidationError

// New creates a new Validator.
func New() *Validator {
	return &Validator{}
}

// Field starts validation for a specific field.
func (v *Validator) Field(name string, value interface{}) *Validator {
	v.rules = append(v.rules, rule{
		field:  name,
		value:  value,
		checks: make([]checkFunc, 0),
	})
	return v
}

// Required adds a required check.
func (v *Validator) Required() *Validator {
	idx := len(v.rules) - 1
	r := &v.rules[idx]
	r.checks = append(r.checks, func(field string, value interface{}) *ValidationError {
		if value == nil {
			return &ValidationError{Field: field, Message: "field is required"}
		}
		v := reflect.ValueOf(value)
		switch v.Kind() {
		case reflect.String:
			if strings.TrimSpace(v.String()) == "" {
				return &ValidationError{Field: field, Message: "field cannot be empty"}
			}
		case reflect.Slice, reflect.Map:
			if v.Len() == 0 {
				return &ValidationError{Field: field, Message: "field cannot be empty"}
			}
		case reflect.Ptr, reflect.Interface:
			if v.IsNil() {
				return &ValidationError{Field: field, Message: "field is required"}
			}
		}
		return nil
	})
	return v
}

// MinLength adds a minimum length check for strings and slices.
func (v *Validator) MinLength(n int) *Validator {
	idx := len(v.rules) - 1
	r := &v.rules[idx]
	r.checks = append(r.checks, func(field string, value interface{}) *ValidationError {
		v := reflect.ValueOf(value)
		switch v.Kind() {
		case reflect.String:
			if v.Len() < n {
				return &ValidationError{
					Field:   field,
					Message: fmt.Sprintf("field must be at least %d characters", n),
				}
			}
		case reflect.Slice, reflect.Map:
			if v.Len() < n {
				return &ValidationError{
					Field:   field,
					Message: fmt.Sprintf("field must have at least %d items", n),
				}
			}
		}
		return nil
	})
	return v
}

// MaxLength adds a maximum length check for strings and slices.
func (v *Validator) MaxLength(n int) *Validator {
	idx := len(v.rules) - 1
	r := &v.rules[idx]
	r.checks = append(r.checks, func(field string, value interface{}) *ValidationError {
		v := reflect.ValueOf(value)
		switch v.Kind() {
		case reflect.String:
			if v.Len() > n {
				return &ValidationError{
					Field:   field,
					Message: fmt.Sprintf("field must be at most %d characters", n),
				}
			}
		case reflect.Slice, reflect.Map:
			if v.Len() > n {
				return &ValidationError{
					Field:   field,
					Message: fmt.Sprintf("field must have at most %d items", n),
				}
			}
		}
		return nil
	})
	return v
}

// InList checks if the value is one of the allowed values.
func (v *Validator) InList(allowed ...string) *Validator {
	idx := len(v.rules) - 1
	r := &v.rules[idx]
	r.checks = append(r.checks, func(field string, value interface{}) *ValidationError {
		if value == nil {
			return nil
		}
		str, ok := value.(string)
		if !ok {
			return nil
		}
		for _, a := range allowed {
			if str == a {
				return nil
			}
		}
		return &ValidationError{
			Field:   field,
			Message: fmt.Sprintf("field must be one of: %s", strings.Join(allowed, ", ")),
		}
	})
	return v
}

// Min adds a minimum value check for numeric types.
func (v *Validator) Min(n float64) *Validator {
	idx := len(v.rules) - 1
	r := &v.rules[idx]
	r.checks = append(r.checks, func(field string, value interface{}) *ValidationError {
		v := reflect.ValueOf(value)
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if v.Int() < int64(n) {
				return &ValidationError{
					Field:   field,
					Message: fmt.Sprintf("field must be at least %v", n),
				}
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if v.Uint() < uint64(n) {
				return &ValidationError{
					Field:   field,
					Message: fmt.Sprintf("field must be at least %v", n),
				}
			}
		case reflect.Float32, reflect.Float64:
			if v.Float() < n {
				return &ValidationError{
					Field:   field,
					Message: fmt.Sprintf("field must be at least %v", n),
				}
			}
		}
		return nil
	})
	return v
}

// Max adds a maximum value check for numeric types.
func (v *Validator) Max(n float64) *Validator {
	idx := len(v.rules) - 1
	r := &v.rules[idx]
	r.checks = append(r.checks, func(field string, value interface{}) *ValidationError {
		v := reflect.ValueOf(value)
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if v.Int() > int64(n) {
				return &ValidationError{
					Field:   field,
					Message: fmt.Sprintf("field must be at most %v", n),
				}
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if v.Uint() > uint64(n) {
				return &ValidationError{
					Field:   field,
					Message: fmt.Sprintf("field must be at most %v", n),
				}
			}
		case reflect.Float32, reflect.Float64:
			if v.Float() > n {
				return &ValidationError{
					Field:   field,
					Message: fmt.Sprintf("field must be at most %v", n),
				}
			}
		}
		return nil
	})
	return v
}

// IsTime checks if the value can be parsed as a time.
func (v *Validator) IsTime(format string) *Validator {
	idx := len(v.rules) - 1
	r := &v.rules[idx]
	r.checks = append(r.checks, func(field string, value interface{}) *ValidationError {
		str, ok := value.(string)
		if !ok {
			return nil
		}
		if _, err := time.Parse(format, str); err != nil {
			return &ValidationError{
				Field:   field,
				Message: fmt.Sprintf("field must be a valid time in format: %s", format),
			}
		}
		return nil
	})
	return v
}

// Validate runs all accumulated rules and returns any errors.
func (v *Validator) Validate() error {
	var errs ValidationErrors
	for _, r := range v.rules {
		for _, check := range r.checks {
			if err := check(r.field, r.value); err != nil {
				errs = append(errs, *err)
			}
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}
