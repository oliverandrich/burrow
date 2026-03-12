package forms

import (
	"reflect"
	"strings"
	"time"
)

// fieldFormName returns the HTML form field name for a struct field.
// Uses the "form" tag if present, otherwise lowercases the Go field name.
func fieldFormName(sf reflect.StructField) string {
	if tag := sf.Tag.Get("form"); tag != "" && tag != "-" {
		return tag
	}
	return strings.ToLower(sf.Name)
}

// parseLabel returns the human-readable label for a struct field.
// Checks "verbose_name" first, then "verbose" as alias, then falls back to the field name.
func parseLabel(sf reflect.StructField) string {
	if vn := sf.Tag.Get("verbose_name"); vn != "" {
		return vn
	}
	if vn := sf.Tag.Get("verbose"); vn != "" {
		return vn
	}
	return sf.Name
}

// parseHelpText returns the help_text tag value, or empty string.
func parseHelpText(sf reflect.StructField) string {
	return sf.Tag.Get("help_text")
}

// parseWidget returns the widget tag value, or empty string.
func parseWidget(sf reflect.StructField) string {
	return sf.Tag.Get("widget")
}

// parseChoices parses the "choices" tag into a slice of Choice.
// Format: "value1|value2|value3" — label equals value.
func parseChoices(sf reflect.StructField) []Choice {
	tag := sf.Tag.Get("choices")
	if tag == "" {
		return nil
	}
	var choices []Choice
	for v := range strings.SplitSeq(tag, "|") {
		choices = append(choices, Choice{Value: v, Label: v})
	}
	return choices
}

// hasRequiredValidation checks if the "validate" tag contains "required".
func hasRequiredValidation(sf reflect.StructField) bool {
	tag := sf.Tag.Get("validate")
	if tag == "" {
		return false
	}
	for rule := range strings.SplitSeq(tag, ",") {
		if rule == "required" {
			return true
		}
	}
	return false
}

// inferType maps a Go type to an HTML input type string.
func inferType(t reflect.Type) string {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	//nolint:exhaustive // only handle common form field types
	switch t.Kind() {
	case reflect.Bool:
		return "checkbox"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number"
	case reflect.String:
		return "text"
	default:
		if t == reflect.TypeFor[time.Time]() {
			return "date"
		}
		return "text"
	}
}

// isSkipped returns true if the field has form:"-".
func isSkipped(sf reflect.StructField) bool {
	return sf.Tag.Get("form") == "-"
}
