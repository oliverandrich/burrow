package modeladmin

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/a-h/templ"
)

// FormField describes a single form field derived from struct introspection.
type FormField struct { //nolint:govet // fieldalignment: readability over optimization
	Name     string   // Go struct field name
	Label    string   // Human-readable label
	Type     string   // "text", "number", "email", "textarea", "select", "checkbox", "date", "hidden"
	Value    any      // Current value
	Required bool     // Whether the field is required
	Choices  []Choice // For select fields
}

// Choice represents a single option in a select field.
type Choice struct {
	Value string
	Label string
}

// AutoFields extracts form fields from a struct using bun and form tags.
// Fields with form:"-" are skipped. Auto-increment PKs are skipped in create mode (item == nil).
func AutoFields[T any](item *T) []FormField {
	var zero T
	t := reflect.TypeOf(zero)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	var v reflect.Value
	if item != nil {
		v = reflect.ValueOf(item).Elem()
	} else {
		v = reflect.New(t).Elem()
	}

	var fields []FormField
	for i := range t.NumField() {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}

		// Skip anonymous (embedded) struct fields like bun.BaseModel.
		if sf.Anonymous {
			continue
		}

		formTag := sf.Tag.Get("form")
		if formTag == "-" {
			continue
		}

		bunTag := sf.Tag.Get("bun")
		isPK := containsOption(bunTag, "pk")
		isAutoIncrement := containsOption(bunTag, "autoincrement")

		// Skip auto-increment PKs in create mode.
		if isPK && isAutoIncrement && item == nil {
			continue
		}

		// Auto-increment PKs are always read-only (hidden in edit mode).
		ff := FormField{
			Name:  sf.Name,
			Label: sf.Name,
			Value: v.Field(i).Interface(),
		}

		if isPK && isAutoIncrement {
			ff.Type = "hidden"
			fields = append(fields, ff)
			continue
		}

		parseFormTag(&ff, formTag)
		if ff.Type == "" {
			ff.Type = inferType(sf.Type)
		}

		fields = append(fields, ff)
	}

	return fields
}

// PopulateFromForm fills a struct's fields from form POST values.
// Only exported, non-skipped fields are populated.
func PopulateFromForm[T any](r *http.Request, item *T) error {
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("parse form: %w", err)
	}

	v := reflect.ValueOf(item).Elem()
	t := v.Type()

	for i := range t.NumField() {
		sf := t.Field(i)
		if !sf.IsExported() || sf.Anonymous {
			continue
		}

		formTag := sf.Tag.Get("form")
		if formTag == "-" {
			continue
		}

		bunTag := sf.Tag.Get("bun")
		if containsOption(bunTag, "pk") && containsOption(bunTag, "autoincrement") {
			continue
		}

		formName := fieldFormName(sf)
		val := r.FormValue(formName)
		if err := setFieldValue(v.Field(i), val); err != nil {
			return fmt.Errorf("field %s: %w", sf.Name, err)
		}
	}

	return nil
}

// fieldFormName returns the form field name for a struct field.
// It uses the form tag if present, otherwise lowercases the field name.
func fieldFormName(sf reflect.StructField) string {
	formTag := sf.Tag.Get("form")
	opts := parseFormTagOptions(formTag)
	if name, ok := opts["name"]; ok {
		return name
	}
	return strings.ToLower(sf.Name)
}

// FormName returns the HTML form field name for a FormField.
func (f FormField) FormName() string {
	return strings.ToLower(f.Name)
}

// parseFormTag parses the form:"..." tag and applies settings to the FormField.
func parseFormTag(ff *FormField, tag string) {
	opts := parseFormTagOptions(tag)

	if label, ok := opts["label"]; ok {
		ff.Label = label
	}
	if widget, ok := opts["widget"]; ok {
		ff.Type = widget
	}
	if choices, ok := opts["choices"]; ok {
		ff.Type = "select"
		for c := range strings.SplitSeq(choices, "|") {
			ff.Choices = append(ff.Choices, Choice{Value: c, Label: c})
		}
	}
	if _, ok := opts["required"]; ok {
		ff.Required = true
	}
}

// parseFormTagOptions parses a form tag into key=value pairs.
// A bare word (no =) is treated as key="" (e.g. "required").
func parseFormTagOptions(tag string) map[string]string {
	opts := make(map[string]string)
	if tag == "" || tag == "-" {
		return opts
	}
	for part := range strings.SplitSeq(tag, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if k, v, ok := strings.Cut(part, "="); ok {
			opts[k] = v
		} else {
			opts[part] = ""
		}
	}
	return opts
}

// inferType guesses the form field type from the Go type.
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

// setFieldValue sets a reflect.Value from a string form value.
func setFieldValue(fv reflect.Value, val string) error {
	if fv.Kind() == reflect.Pointer {
		if val == "" {
			fv.Set(reflect.Zero(fv.Type()))
			return nil
		}
		ptr := reflect.New(fv.Type().Elem())
		if err := setFieldValue(ptr.Elem(), val); err != nil {
			return err
		}
		fv.Set(ptr)
		return nil
	}

	//nolint:exhaustive // only handle common form field types
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(val)
	case reflect.Bool:
		fv.SetBool(val == "on" || val == "true" || val == "1")
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := parseInt(val)
		if err != nil {
			return err
		}
		fv.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := parseUint(val)
		if err != nil {
			return err
		}
		fv.SetUint(n)
	case reflect.Float32, reflect.Float64:
		n, err := parseFloat(val)
		if err != nil {
			return err
		}
		fv.SetFloat(n)
	default:
		if fv.Type() == reflect.TypeFor[time.Time]() {
			t, err := time.Parse("2006-01-02", val)
			if err != nil {
				return fmt.Errorf("invalid date %q: %w", val, err)
			}
			fv.Set(reflect.ValueOf(t))
		}
	}

	return nil
}

func parseInt(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

func parseUint(s string) (uint64, error) {
	if s == "" {
		return 0, nil
	}
	var n uint64
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

func parseFloat(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	var n float64
	_, err := fmt.Sscanf(s, "%g", &n)
	return n, err
}

// containsOption checks if a comma-separated tag string contains an option.
func containsOption(tag, option string) bool {
	for part := range strings.SplitSeq(tag, ",") {
		if strings.TrimSpace(part) == option {
			return true
		}
	}
	return false
}

// pkFieldName returns the struct field name tagged as the primary key.
func pkFieldName[T any]() string {
	var zero T
	t := reflect.TypeOf(zero)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	for i := range t.NumField() {
		sf := t.Field(i)
		if containsOption(sf.Tag.Get("bun"), "pk") {
			return sf.Name
		}
	}
	return ""
}

// FieldValue extracts a field value from a struct by field name.
// Returns the value as any, or nil if the field is not found.
func FieldValue(item any, field string) any {
	v := reflect.ValueOf(item)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	fv := v.FieldByName(field)
	if !fv.IsValid() {
		return nil
	}
	return fv.Interface()
}

// columnComponent creates a templ component that renders the value of a struct field.
func columnComponent(item any, field string) templ.Component {
	v := reflect.ValueOf(item)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	fv := v.FieldByName(field)
	if !fv.IsValid() {
		return templ.Raw("<span>-</span>")
	}

	val := fv.Interface()

	// Handle pointer types.
	if fv.Kind() == reflect.Pointer {
		if fv.IsNil() {
			return templ.Raw("<span>-</span>")
		}
		val = fv.Elem().Interface()
	}

	// Format time values.
	if t, ok := val.(time.Time); ok {
		if t.IsZero() {
			return templ.Raw("<span>-</span>")
		}
		return templ.Raw(fmt.Sprintf("<span>%s</span>", t.Format("2006-01-02 15:04")))
	}

	return templ.Raw(fmt.Sprintf("<span>%v</span>", val))
}
