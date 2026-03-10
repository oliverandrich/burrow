package burrow

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/form/v4"
	"github.com/go-playground/validator/v10"
)

// formDecoder decodes url.Values into structs using "form" struct tags.
var formDecoder = form.NewDecoder()

// structValidator validates structs using "validate" struct tags.
var structValidator = newValidator()

func newValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		// Prefer "form" tag, fall back to "json" tag, then Go field name.
		if name := fld.Tag.Get("form"); name != "" && name != "-" {
			return name
		}
		if name := fld.Tag.Get("json"); name != "" && name != "-" {
			if idx := strings.IndexByte(name, ','); idx != -1 {
				name = name[:idx]
			}
			return name
		}
		return fld.Name
	})
	return v
}

// FieldError represents a validation failure on a single field.
type FieldError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Param   string `json:"param,omitempty"`
	Value   any    `json:"value"`
	Message string `json:"message"`
}

// ValidationError is returned by Bind()/Validate() when validation fails.
type ValidationError struct {
	Errors []FieldError
}

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Errors))
	for i, fe := range e.Errors {
		parts[i] = fmt.Sprintf("%s is %s", fe.Field, fe.Tag)
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

// Translate translates field error messages using the given translation function.
// The translateData function receives a key and template data, and returns the
// translated string. Typically called with i18n.TData:
//
//	ve.Translate(ctx, i18n.TData)
func (e *ValidationError) Translate(ctx context.Context, translateData func(context.Context, string, map[string]any) string) {
	for i := range e.Errors {
		fe := &e.Errors[i]
		key := "validation-" + fe.Tag
		data := map[string]any{"Field": fe.Field, "Param": fe.Param}
		translated := translateData(ctx, key, data)
		if translated != key {
			fe.Message = translated
		}
	}
}

// HasField reports whether the validation error contains a failure for the named field.
func (e *ValidationError) HasField(name string) bool {
	for _, fe := range e.Errors {
		if fe.Field == name {
			return true
		}
	}
	return false
}

// Validate validates a struct using "validate" struct tags.
// Returns nil if v is not a struct, has no validate tags, or passes all checks.
func Validate(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil
	}

	err := structValidator.Struct(v)
	if err == nil {
		return nil
	}
	return toValidationError(err)
}

func toValidationError(err error) error {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return err
	}
	fields := make([]FieldError, len(ve))
	for i, fe := range ve {
		fields[i] = FieldError{
			Field:   fe.Field(),
			Tag:     fe.Tag(),
			Param:   fe.Param(),
			Value:   fe.Value(),
			Message: fieldErrorMessage(fe),
		}
	}
	return &ValidationError{Errors: fields}
}

func fieldErrorMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", fe.Field())
	case "email":
		return fmt.Sprintf("%s must be a valid email address", fe.Field())
	case "min":
		return fmt.Sprintf("%s must be at least %s", fe.Field(), fe.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s", fe.Field(), fe.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", fe.Field(), fe.Param())
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", fe.Field(), fe.Param())
	case "len":
		return fmt.Sprintf("%s must be exactly %s characters", fe.Field(), fe.Param())
	case "url":
		return fmt.Sprintf("%s must be a valid URL", fe.Field())
	default:
		return fmt.Sprintf("%s failed %s validation", fe.Field(), fe.Tag())
	}
}
