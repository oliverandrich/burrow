package forms

import "context"

// formConfig holds configuration for a Form, populated by Option functions.
type formConfig[T any] struct {
	initial   map[string]any
	choices   map[string][]Choice
	choiceFns map[string]func(context.Context) ([]Choice, error)
	exclude   map[string]struct{}
	readOnly  map[string]struct{}
}

// Option configures a Form during construction.
type Option[T any] func(*formConfig[T])

// WithInitial sets initial field values by form name.
func WithInitial[T any](values map[string]any) Option[T] {
	return func(cfg *formConfig[T]) {
		cfg.initial = values
	}
}

// WithChoices sets static choices for a field (by Go struct field name).
func WithChoices[T any](field string, choices []Choice) Option[T] {
	return func(cfg *formConfig[T]) {
		if cfg.choices == nil {
			cfg.choices = make(map[string][]Choice)
		}
		cfg.choices[field] = choices
	}
}

// WithChoicesFunc sets a dynamic choice provider for a field (by Go struct field name).
func WithChoicesFunc[T any](field string, fn func(context.Context) ([]Choice, error)) Option[T] {
	return func(cfg *formConfig[T]) {
		if cfg.choiceFns == nil {
			cfg.choiceFns = make(map[string]func(context.Context) ([]Choice, error))
		}
		cfg.choiceFns[field] = fn
	}
}

// WithExclude excludes named fields (by Go struct field name) from the form.
func WithExclude[T any](fields ...string) Option[T] {
	return func(cfg *formConfig[T]) {
		if cfg.exclude == nil {
			cfg.exclude = make(map[string]struct{}, len(fields))
		}
		for _, f := range fields {
			cfg.exclude[f] = struct{}{}
		}
	}
}

// WithReadOnly marks named fields (by Go struct field name) as read-only.
// Read-only fields are rendered as plain text instead of input elements.
func WithReadOnly[T any](fields ...string) Option[T] {
	return func(cfg *formConfig[T]) {
		if cfg.readOnly == nil {
			cfg.readOnly = make(map[string]struct{}, len(fields))
		}
		for _, f := range fields {
			cfg.readOnly[f] = struct{}{}
		}
	}
}
