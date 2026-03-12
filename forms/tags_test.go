package forms

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type tagTestStruct struct { //nolint:govet // fieldalignment: readability over optimization
	Name    string `form:"name" verbose_name:"Full Name" help_text:"Enter your full name" widget:"text" validate:"required"`
	Bio     string `widget:"textarea" verbose:"Biography"`
	Email   string `form:"email_addr" widget:"email"`
	Age     int
	Hidden  string `form:"-"`
	Status  string `choices:"draft|published|archived"`
	NoLabel string `form:"no_label"`
}

func fieldByName(t reflect.Type, name string) reflect.StructField {
	sf, ok := t.FieldByName(name)
	if !ok {
		panic("field not found: " + name)
	}
	return sf
}

func TestFieldFormName(t *testing.T) {
	typ := reflect.TypeFor[tagTestStruct]()

	tests := []struct {
		field string
		want  string
	}{
		{"Name", "name"},
		{"Bio", "bio"},          // no form tag → lowercase
		{"Email", "email_addr"}, // explicit form tag
		{"Age", "age"},          // no tags → lowercase
		{"NoLabel", "no_label"},
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			sf := fieldByName(typ, tt.field)
			assert.Equal(t, tt.want, fieldFormName(sf))
		})
	}
}

func TestParseLabel(t *testing.T) {
	typ := reflect.TypeFor[tagTestStruct]()

	tests := []struct {
		field string
		want  string
	}{
		{"Name", "Full Name"},  // verbose_name tag
		{"Bio", "Biography"},   // verbose tag (alias)
		{"Email", "Email"},     // no verbose tag → field name
		{"Age", "Age"},         // no tags → field name
		{"NoLabel", "NoLabel"}, // no verbose → field name
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			sf := fieldByName(typ, tt.field)
			assert.Equal(t, tt.want, parseLabel(sf))
		})
	}
}

func TestParseHelpText(t *testing.T) {
	typ := reflect.TypeFor[tagTestStruct]()

	assert.Equal(t, "Enter your full name", parseHelpText(fieldByName(typ, "Name")))
	assert.Empty(t, parseHelpText(fieldByName(typ, "Bio")))
}

func TestParseWidget(t *testing.T) {
	typ := reflect.TypeFor[tagTestStruct]()

	tests := []struct {
		field string
		want  string
	}{
		{"Name", "text"},
		{"Bio", "textarea"},
		{"Email", "email"},
		{"Age", ""},    // no widget tag
		{"Status", ""}, // no widget tag (choices sets type separately)
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			sf := fieldByName(typ, tt.field)
			assert.Equal(t, tt.want, parseWidget(sf))
		})
	}
}

func TestParseChoices(t *testing.T) {
	typ := reflect.TypeFor[tagTestStruct]()

	choices := parseChoices(fieldByName(typ, "Status"))
	assert.Len(t, choices, 3)
	assert.Equal(t, "draft", choices[0].Value)
	assert.Equal(t, "draft", choices[0].Label)
	assert.Equal(t, "published", choices[1].Value)
	assert.Equal(t, "archived", choices[2].Value)

	// No choices tag.
	assert.Empty(t, parseChoices(fieldByName(typ, "Name")))
}

func TestHasRequiredValidation(t *testing.T) {
	typ := reflect.TypeFor[tagTestStruct]()

	assert.True(t, hasRequiredValidation(fieldByName(typ, "Name")))
	assert.False(t, hasRequiredValidation(fieldByName(typ, "Bio")))
	assert.False(t, hasRequiredValidation(fieldByName(typ, "Age")))
}

func TestHasRequiredValidationComplex(t *testing.T) {
	type s struct {
		A string `validate:"required,email"`
		B string `validate:"min=3,required,max=10"`
		C string `validate:"email"`
	}
	typ := reflect.TypeFor[s]()

	assert.True(t, hasRequiredValidation(fieldByName(typ, "A")))
	assert.True(t, hasRequiredValidation(fieldByName(typ, "B")))
	assert.False(t, hasRequiredValidation(fieldByName(typ, "C")))
}

func TestInferType(t *testing.T) {
	tests := []struct {
		name string
		typ  reflect.Type
		want string
	}{
		{"string", reflect.TypeFor[string](), "text"},
		{"int", reflect.TypeFor[int](), "number"},
		{"int64", reflect.TypeFor[int64](), "number"},
		{"float64", reflect.TypeFor[float64](), "number"},
		{"bool", reflect.TypeFor[bool](), "checkbox"},
		{"time", reflect.TypeFor[time.Time](), "date"},
		{"*string", reflect.TypeFor[*string](), "text"},
		{"*int", reflect.TypeFor[*int](), "number"},
		{"*bool", reflect.TypeFor[*bool](), "checkbox"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, inferType(tt.typ))
		})
	}
}

func TestIsSkipped(t *testing.T) {
	typ := reflect.TypeFor[tagTestStruct]()

	assert.True(t, isSkipped(fieldByName(typ, "Hidden")))
	assert.False(t, isSkipped(fieldByName(typ, "Name")))
}
