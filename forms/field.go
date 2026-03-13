package forms

import (
	"context"
	"reflect"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/i18n"
)

// BoundField provides field metadata for template rendering.
type BoundField struct { //nolint:govet // fieldalignment: readability over optimization
	Name     string   // Go struct field name
	FormName string   // HTML field name (from form tag or lowercase)
	Label    string   // from verbose_name/verbose tag
	HelpText string   // from help_text tag
	Type     string   // "text", "number", "textarea", "select", "checkbox", "date", "email", "hidden"
	Value    any      // current value
	Required bool     // from validate:"required"
	ReadOnly bool     // render as plain text, not editable
	Choices  []Choice // static or dynamic
	Errors   []string // field-specific error messages
}

// Choice represents a single option in a select or radio field.
type Choice struct {
	Value    string
	Label    string
	LabelKey string // optional i18n key
}

// extractFields builds a slice of BoundField from a struct instance,
// merging validation errors and dynamic choices. Fields in the exclude set
// (keyed by Go struct field name) are omitted.
func extractFields[T any](ctx context.Context, instance *T, validationErr *burrow.ValidationError, choices map[string][]Choice, exclude, readOnly map[string]struct{}) []BoundField {
	v := reflect.ValueOf(instance).Elem()
	t := v.Type()

	var fields []BoundField
	for i := range t.NumField() {
		sf := t.Field(i)
		if !sf.IsExported() || sf.Anonymous {
			continue
		}
		if isSkipped(sf) {
			// ReadOnly overrides form:"-" — the field is shown but not editable.
			if _, ok := readOnly[sf.Name]; !ok {
				continue
			}
		}
		if _, ok := exclude[sf.Name]; ok {
			continue
		}

		_, isReadOnly := readOnly[sf.Name]
		bf := BoundField{
			Name:     sf.Name,
			FormName: fieldFormName(sf),
			Label:    parseLabel(sf),
			HelpText: parseHelpText(sf),
			Value:    fieldValue(v.Field(i)),
			Required: hasRequiredValidation(sf),
			ReadOnly: isReadOnly,
		}

		// Determine type: widget tag > choices > inferred from Go type.
		if w := parseWidget(sf); w != "" {
			bf.Type = w
		} else if c := parseChoices(sf); len(c) > 0 {
			bf.Type = "select"
			bf.Choices = c
		} else {
			bf.Type = inferType(sf.Type)
		}

		// Override choices from dynamic source.
		if dc, ok := choices[sf.Name]; ok {
			bf.Choices = dc
			if bf.Type != "select" {
				bf.Type = "select"
			}
		}

		// Collect field errors and translate any i18n keys.
		if validationErr != nil {
			bf.Errors = fieldErrors(ctx, validationErr, bf.FormName)
		}

		fields = append(fields, bf)
	}

	return fields
}

// fieldValue returns the value for template rendering, dereferencing pointers.
func fieldValue(fv reflect.Value) any {
	if fv.Kind() == reflect.Pointer {
		if fv.IsNil() {
			return nil
		}
		return fv.Elem().Interface()
	}
	return fv.Interface()
}

// fieldErrors returns translated error messages for a specific field.
// Tag-based errors (from validation) are translated via "validation-{tag}" keys
// with template data. Custom messages (from Clean/WithCleanFunc) are translated
// as plain i18n keys.
func fieldErrors(ctx context.Context, ve *burrow.ValidationError, formName string) []string {
	var errs []string
	for _, fe := range ve.Errors {
		if fe.Field != formName {
			continue
		}
		if fe.Tag != "" {
			errs = append(errs, translateTagError(ctx, fe))
		} else {
			errs = append(errs, i18n.T(ctx, fe.Message))
		}
	}
	return errs
}

// translateTagError translates a validation-tag-based error using i18n.TData.
func translateTagError(ctx context.Context, fe burrow.FieldError) string {
	key := "validation-" + fe.Tag
	data := map[string]any{"Field": fe.Field, "Param": fe.Param}
	translated := i18n.TData(ctx, key, data)
	if translated != key {
		return translated
	}
	return fe.Message
}
