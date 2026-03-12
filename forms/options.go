package forms

import "context"

// TranslateFunc translates a message key with template data for a given context.
// Compatible with i18n.TData.
type TranslateFunc func(context.Context, string, map[string]any) string

// formConfig holds configuration for a Form, populated by Option functions.
type formConfig[T any] struct {
	initial     map[string]any
	choices     map[string][]Choice
	choiceFns   map[string]func(context.Context) ([]Choice, error)
	exclude     map[string]struct{}
	translateFn TranslateFunc
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

// WithTranslateFunc sets a translation function for validation error messages.
// Pass i18n.TData to auto-translate errors using the request's locale.
func WithTranslateFunc[T any](fn TranslateFunc) Option[T] {
	return func(cfg *formConfig[T]) {
		cfg.translateFn = fn
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
