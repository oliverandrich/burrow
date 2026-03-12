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
func extractFields[T any](ctx context.Context, instance *T, validationErr *burrow.ValidationError, choices map[string][]Choice, exclude map[string]struct{}) []BoundField {
	// Auto-translate validation errors using i18n.TData.
	if validationErr != nil && ctx != nil {
		validationErr.Translate(ctx, i18n.TData)
	}

	v := reflect.ValueOf(instance).Elem()
	t := v.Type()

	var fields []BoundField
	for i := range t.NumField() {
		sf := t.Field(i)
		if !sf.IsExported() || sf.Anonymous {
			continue
		}
		if isSkipped(sf) {
			continue
		}
		if _, ok := exclude[sf.Name]; ok {
			continue
		}

		bf := BoundField{
			Name:     sf.Name,
			FormName: fieldFormName(sf),
			Label:    parseLabel(sf),
			HelpText: parseHelpText(sf),
			Value:    v.Field(i).Interface(),
			Required: hasRequiredValidation(sf),
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

		// Collect field errors.
		if validationErr != nil {
			bf.Errors = fieldErrors(validationErr, bf.FormName)
		}

		fields = append(fields, bf)
	}

	return fields
}

// fieldErrors returns error messages for a specific field from a ValidationError.
func fieldErrors(ve *burrow.ValidationError, formName string) []string {
	var errs []string
	for _, fe := range ve.Errors {
		if fe.Field == formName {
			errs = append(errs, fe.Message)
		}
	}
	return errs
}
