package forms

import (
	"context"
	"reflect"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/i18n"
)

// BoundField provides field metadata for template rendering.
//
// Label is read from the verbose_name/verbose struct tag and piped through
// [i18n.T] by [extractFields], so the English Label doubles as the i18n
// message ID. Templates render {{ .Label }} as-is — no {{ t }} wrapping
// needed. See docs/guide/i18n.md for the Label-as-key convention.
//
// When Type is "subform", SubFields holds the nested struct's BoundFields
// (one level of recursion). Their FormName uses the parent.child convention
// (e.g. "profile.name"), which matches the [burrow.Bind] decoder.
type BoundField struct { //nolint:govet // fieldalignment: readability over optimization
	Name      string       // Go struct field name
	FormName  string       // HTML field name (from form tag or lowercase)
	Label     string       // translated from verbose_name/verbose tag
	HelpText  string       // from help_text tag
	Type      string       // "text", "number", "textarea", "select", "checkbox", "date", "email", "hidden", "subform"
	Value     any          // current value
	Required  bool         // from validate:"required"
	ReadOnly  bool         // render as plain text, not editable
	Choices   []Choice     // static or dynamic, with translated labels
	SubFields []BoundField // populated when Type == "subform"
	Errors    []string     // field-specific error messages
}

// Choice represents a single option in a select or radio field. Label is
// piped through [i18n.T] by [extractFields], following the same
// Label-as-key convention as [BoundField.Label].
type Choice struct {
	Value string
	Label string
}

// extractFields builds a slice of BoundField from a struct instance,
// merging validation errors and dynamic choices. Fields in the exclude set
// (keyed by Go struct field name) are omitted.
func extractFields[T any](ctx context.Context, instance *T, validationErr *burrow.ValidationError, choices map[string][]Choice, exclude, readOnly map[string]struct{}) []BoundField {
	v := reflect.ValueOf(instance).Elem()
	return walkStructFields(ctx, v, validationErr, "", choices, exclude, readOnly, true)
}

// walkStructFields builds the BoundField slice for a struct value. When
// allowSubforms is true, struct-typed fields recurse one level (their
// SubFields are populated and FormNames are prefixed by the parent's name).
// On the recursive call allowSubforms is false, capping nesting at one level.
//
// formNamePrefix is the parent's FormName when recursing (empty at top
// level); nested FormNames follow the parent.child convention so they match
// validation errors emitted by burrow.Bind.
func walkStructFields(
	ctx context.Context,
	v reflect.Value,
	validationErr *burrow.ValidationError,
	formNamePrefix string,
	choices map[string][]Choice,
	exclude, readOnly map[string]struct{},
	allowSubforms bool,
) []BoundField {
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
		formName := fieldFormName(sf)
		if formNamePrefix != "" {
			formName = formNamePrefix + "." + formName
		}
		bf := BoundField{
			Name:     sf.Name,
			FormName: formName,
			Label:    i18n.T(ctx, parseLabel(sf)),
			HelpText: parseHelpText(sf),
			Value:    fieldValue(v.Field(i)),
			Required: hasRequiredValidation(sf),
			ReadOnly: isReadOnly,
		}

		// Determine type: widget tag > choices > subform (one level) > inferred.
		widget := parseWidget(sf)
		tagChoices := parseChoices(sf)
		switch {
		case widget != "":
			bf.Type = widget
		case len(tagChoices) > 0:
			bf.Type = "select"
			bf.Choices = tagChoices
		case allowSubforms && isSubformType(sf.Type):
			bf.Type = "subform"
			bf.SubFields = walkStructFields(ctx, subformValue(v.Field(i)), validationErr, formName, nil, nil, nil, false)
		default:
			bf.Type = inferType(sf.Type)
		}

		// Override choices from dynamic source.
		if dc, ok := choices[sf.Name]; ok {
			bf.Choices = dc
			if bf.Type != "select" {
				bf.Type = "select"
			}
		}

		// Translate Choice labels through the same Label-as-key convention as
		// BoundField.Label. Clone first — WithChoices and WithChoicesFunc may
		// hand us a slice that lives on the Form config (or a package-level
		// variable in the caller), so mutating in place would bleed translated
		// values back into the source and corrupt subsequent renders.
		if len(bf.Choices) > 0 {
			translated := make([]Choice, len(bf.Choices))
			for j, c := range bf.Choices {
				translated[j] = Choice{Value: c.Value, Label: i18n.T(ctx, c.Label)}
			}
			bf.Choices = translated
		}

		// Collect field errors and translate any i18n keys.
		if validationErr != nil {
			bf.Errors = fieldErrors(ctx, validationErr, bf.FormName)
		}

		fields = append(fields, bf)
	}

	return fields
}

// subformValue returns the struct reflect.Value to recurse into. A nil
// pointer is replaced with a zero value of the pointee so the sub-fields
// render as empty inputs instead of erroring out.
func subformValue(fv reflect.Value) reflect.Value {
	if fv.Kind() == reflect.Pointer {
		if fv.IsNil() {
			return reflect.New(fv.Type().Elem()).Elem()
		}
		return fv.Elem()
	}
	return fv
}

// fieldValue returns the value for template rendering, dereferencing pointers.
// Nil pointers return the zero value of the element type (e.g. "" for *string).
func fieldValue(fv reflect.Value) any {
	if fv.Kind() == reflect.Pointer {
		if fv.IsNil() {
			return reflect.Zero(fv.Type().Elem()).Interface()
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
