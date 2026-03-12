package modeladmin

import (
	"fmt"
	"html"
	"html/template"
	"reflect"
	"strings"
	"time"
)

// containsOption checks if a comma-separated tag string contains an option.
func containsOption(tag, option string) bool {
	for part := range strings.SplitSeq(tag, ",") {
		if strings.TrimSpace(part) == option {
			return true
		}
	}
	return false
}

// tableName extracts the Bun table name from the bun:"table:..." struct tag
// on the embedded bun.BaseModel field. Returns "" if not found.
func tableName[T any]() string {
	var zero T
	t := reflect.TypeOf(zero)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	for i := range t.NumField() {
		sf := t.Field(i)
		if !sf.Anonymous {
			continue
		}
		bunTag := sf.Tag.Get("bun")
		for part := range strings.SplitSeq(bunTag, ",") {
			part = strings.TrimSpace(part)
			if name, ok := strings.CutPrefix(part, "table:"); ok {
				return name
			}
		}
	}
	return ""
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

// verboseNames extracts verbose:"..." tags from struct fields,
// returning a map of field name → verbose name.
func verboseNames[T any]() map[string]string {
	var zero T
	t := reflect.TypeOf(zero)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	names := make(map[string]string)
	for i := range t.NumField() {
		sf := t.Field(i)
		if !sf.IsExported() || sf.Anonymous {
			continue
		}
		if vn := sf.Tag.Get("verbose"); vn != "" {
			names[sf.Name] = vn
		}
	}
	return names
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

// columnHTML renders the value of a struct field as safe HTML.
func columnHTML(item any, field string, t func(string) string) template.HTML {
	v := reflect.ValueOf(item)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	fv := v.FieldByName(field)
	if !fv.IsValid() {
		return "<span>-</span>"
	}

	val := fv.Interface()

	// Handle pointer types.
	if fv.Kind() == reflect.Pointer {
		if fv.IsNil() {
			return "<span>-</span>"
		}
		val = fv.Elem().Interface()
	}

	// Format bool values with i18n.
	if b, ok := val.(bool); ok {
		key := "modeladmin-no"
		if b {
			key = "modeladmin-yes"
		}
		label := key
		if t != nil {
			label = t(key)
		}
		return template.HTML("<span>" + html.EscapeString(label) + "</span>") //nolint:gosec // value is escaped
	}

	// Format time values.
	if tm, ok := val.(time.Time); ok {
		if tm.IsZero() {
			return "<span>-</span>"
		}
		return template.HTML("<span>" + tm.Format("2006-01-02 15:04") + "</span>") //nolint:gosec // time format is safe
	}

	return template.HTML("<span>" + html.EscapeString(fmt.Sprintf("%v", val)) + "</span>") //nolint:gosec // value is escaped
}
