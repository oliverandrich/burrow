// Package forms provides Django-style form structs for creation, binding,
// validation, and field metadata extraction.
package forms

import "context"

// ChoiceProvider may be implemented by form structs that provide dynamic
// choices for select fields. The field parameter is the Go struct field name.
type ChoiceProvider interface {
	FieldChoices(ctx context.Context, field string) ([]Choice, error)
}

// Cleanable may be implemented by form structs for cross-field validation.
// Clean is called after per-field validation passes. The context carries
// request-scoped data (e.g. i18n localizer). It may return a
// *burrow.ValidationError to report field-level or non-field errors.
type Cleanable interface {
	Clean(ctx context.Context) error
}
