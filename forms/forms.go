package forms

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"reflect"

	"github.com/oliverandrich/burrow"
)

// Form holds form state for a struct type T.
type Form[T any] struct { //nolint:govet // fieldalignment: readability over optimization
	instance *T
	config   formConfig[T]
	ctx      context.Context
	errs     *burrow.ValidationError
	nonField []string
	bound    bool
	valid    bool
	choices  map[string][]Choice
}

// New creates an empty form for type T.
func New[T any](opts ...Option[T]) *Form[T] {
	f := &Form[T]{
		instance: new(T),
	}
	for _, opt := range opts {
		opt(&f.config)
	}
	applyInitial(f.instance, f.config.initial)
	applyStaticChoices(f)
	return f
}

// FromModel creates a form pre-populated from an existing model instance.
// If instance is nil, creates an empty form (for create mode).
func FromModel[T any](instance *T, opts ...Option[T]) *Form[T] {
	f := &Form[T]{}
	if instance != nil {
		cp := *instance
		f.instance = &cp
	} else {
		f.instance = new(T)
	}
	for _, opt := range opts {
		opt(&f.config)
	}
	applyStaticChoices(f)
	return f
}

// Bind decodes the request body into the form struct, validates it,
// and runs any Cleanable.Clean method. Returns true if the form is valid.
func (f *Form[T]) Bind(r *http.Request) bool {
	f.bound = true
	f.ctx = r.Context()
	f.errs = nil
	f.nonField = nil

	// Decode + validate via burrow.Bind.
	if err := burrow.Bind(r, f.instance); err != nil {
		var ve *burrow.ValidationError
		if errors.As(err, &ve) {
			f.errs = ve
		} else {
			// Decode error — treat as non-field error.
			f.nonField = append(f.nonField, err.Error())
			f.valid = false
			return false
		}
	}

	// Load dynamic choices from ChoiceProvider interface.
	if cp, ok := any(f.instance).(ChoiceProvider); ok {
		f.loadChoiceProvider(r, cp)
	}

	// Load dynamic choices from WithChoicesFunc options.
	f.loadChoiceFuncs(r)

	// Run cross-field validation if the struct implements Cleanable.
	if f.errs == nil {
		if c, ok := any(f.instance).(Cleanable); ok {
			if err := c.Clean(); err != nil {
				f.mergeCleanErrors(err)
			}
		}
	}

	f.valid = f.errs == nil && len(f.nonField) == 0
	return f.valid
}

// IsValid reports whether the form passed validation.
func (f *Form[T]) IsValid() bool {
	return f.valid
}

// Instance returns the bound/populated struct.
func (f *Form[T]) Instance() *T {
	return f.instance
}

// Errors returns the validation errors, or nil if valid.
func (f *Form[T]) Errors() *burrow.ValidationError {
	return f.errs
}

// NonFieldErrors returns errors not tied to a specific field (from Clean).
func (f *Form[T]) NonFieldErrors() []string {
	return f.nonField
}

// Fields returns all visible BoundFields in struct field order.
// Validation errors are auto-translated via i18n.TData.
func (f *Form[T]) Fields() []BoundField {
	return extractFields(f.ctx, f.instance, f.errs, f.choices, f.config.exclude)
}

// Field returns a single BoundField by Go struct field name.
func (f *Form[T]) Field(name string) (BoundField, bool) {
	for _, bf := range f.Fields() {
		if bf.Name == name {
			return bf, true
		}
	}
	return BoundField{}, false
}

// loadChoiceProvider loads choices from the ChoiceProvider interface.
func (f *Form[T]) loadChoiceProvider(r *http.Request, cp ChoiceProvider) {
	t := reflect.TypeFor[T]()
	for i := range t.NumField() {
		sf := t.Field(i)
		if !sf.IsExported() || sf.Anonymous || isSkipped(sf) {
			continue
		}
		choices, err := cp.FieldChoices(r.Context(), sf.Name)
		if err != nil || len(choices) == 0 {
			continue
		}
		if f.choices == nil {
			f.choices = make(map[string][]Choice)
		}
		f.choices[sf.Name] = choices
	}
}

// loadChoiceFuncs loads choices from WithChoicesFunc options.
func (f *Form[T]) loadChoiceFuncs(r *http.Request) {
	for field, fn := range f.config.choiceFns {
		choices, err := fn(r.Context())
		if err != nil || len(choices) == 0 {
			continue
		}
		if f.choices == nil {
			f.choices = make(map[string][]Choice)
		}
		f.choices[field] = choices
	}
}

// mergeCleanErrors merges errors from Clean() into the form's error state.
func (f *Form[T]) mergeCleanErrors(err error) {
	var ve *burrow.ValidationError
	if errors.As(err, &ve) {
		for _, fe := range ve.Errors {
			if fe.Field == "" {
				f.nonField = append(f.nonField, fe.Message)
			} else {
				if f.errs == nil {
					f.errs = &burrow.ValidationError{}
				}
				f.errs.Errors = append(f.errs.Errors, fe)
			}
		}
	} else {
		f.nonField = append(f.nonField, err.Error())
	}
}

// applyInitial sets initial values on the instance from a map.
func applyInitial[T any](instance *T, initial map[string]any) {
	if len(initial) == 0 {
		return
	}
	v := reflect.ValueOf(instance).Elem()
	t := v.Type()
	for i := range t.NumField() {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}
		formName := fieldFormName(sf)
		val, ok := initial[formName]
		if !ok {
			continue
		}
		fv := v.Field(i)
		rv := reflect.ValueOf(val)
		if rv.Type().AssignableTo(fv.Type()) {
			fv.Set(rv)
		}
	}
}

// applyStaticChoices copies WithChoices options into the form's choices map.
func applyStaticChoices[T any](f *Form[T]) {
	if len(f.config.choices) == 0 {
		return
	}
	if f.choices == nil {
		f.choices = make(map[string][]Choice)
	}
	maps.Copy(f.choices, f.config.choices)
}
