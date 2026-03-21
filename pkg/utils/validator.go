package utils

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/microcosm-cc/bluemonday"
	"github.com/zatrano/zbe/internal/domain"
)

var (
	validate     *validator.Validate
	sanitizer    *bluemonday.Policy
	validatorOnce sync.Once
)

// initValidator lazily initialises the validator singleton.
func initValidator() {
	validatorOnce.Do(func() {
		validate = validator.New()

		// Use JSON field names in validation errors instead of struct field names.
		validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			return name
		})

		sanitizer = bluemonday.StrictPolicy()
	})
}

// ValidateStruct validates a struct using go-playground/validator tags.
// Returns a slice of ValidationError if invalid, nil if valid.
func ValidateStruct(s interface{}) []domain.ValidationError {
	initValidator()
	err := validate.Struct(s)
	if err == nil {
		return nil
	}

	var errs []domain.ValidationError
	for _, e := range err.(validator.ValidationErrors) {
		errs = append(errs, domain.ValidationError{
			Field:   e.Field(),
			Message: formatValidationMessage(e),
		})
	}
	return errs
}

// SanitizeString removes all HTML from a string.
func SanitizeString(s string) string {
	initValidator()
	return sanitizer.Sanitize(s)
}

// SanitizeStringPtr sanitises a *string in place.
func SanitizeStringPtr(s *string) {
	if s == nil {
		return
	}
	*s = SanitizeString(*s)
}

// formatValidationMessage converts a validator FieldError to a human-readable message.
func formatValidationMessage(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", e.Field())
	case "email":
		return fmt.Sprintf("%s must be a valid email address", e.Field())
	case "min":
		if e.Kind() == reflect.String {
			return fmt.Sprintf("%s must be at least %s characters", e.Field(), e.Param())
		}
		return fmt.Sprintf("%s must be at least %s", e.Field(), e.Param())
	case "max":
		if e.Kind() == reflect.String {
			return fmt.Sprintf("%s must be at most %s characters", e.Field(), e.Param())
		}
		return fmt.Sprintf("%s must be at most %s", e.Field(), e.Param())
	case "url":
		return fmt.Sprintf("%s must be a valid URL", e.Field())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", e.Field(), e.Param())
	case "uuid":
		return fmt.Sprintf("%s must be a valid UUID", e.Field())
	default:
		return fmt.Sprintf("%s is invalid (%s)", e.Field(), e.Tag())
	}
}
