package forms

import (
	"context"
	"testing"

	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var noCtx = context.Background() //nolint:gochecknoglobals // test helper

type articleForm struct { //nolint:govet // fieldalignment: readability over optimization
	Title   string `form:"title" verbose_name:"Title" validate:"required"`
	Content string `form:"content" verbose_name:"Content" widget:"textarea"`
	Status  string `form:"status" choices:"draft|published"`
	Views   int    `form:"views"`
	Hidden  string `form:"-"`
}

func TestExtractFieldsBasic(t *testing.T) {
	instance := &articleForm{
		Title:   "Hello",
		Content: "World",
		Status:  "draft",
		Views:   42,
	}

	fields := extractFields(noCtx, nil, instance, nil, nil, nil)

	require.Len(t, fields, 4) // Hidden is skipped

	assert.Equal(t, "Title", fields[0].Name)
	assert.Equal(t, "title", fields[0].FormName)
	assert.Equal(t, "Title", fields[0].Label)
	assert.Equal(t, "text", fields[0].Type) // no widget, string → text
	assert.Equal(t, "Hello", fields[0].Value)
	assert.True(t, fields[0].Required)
	assert.Empty(t, fields[0].Errors)

	assert.Equal(t, "Content", fields[1].Name)
	assert.Equal(t, "textarea", fields[1].Type) // widget tag
	assert.Equal(t, "World", fields[1].Value)
	assert.False(t, fields[1].Required)

	assert.Equal(t, "Status", fields[2].Name)
	assert.Equal(t, "select", fields[2].Type) // choices tag
	assert.Len(t, fields[2].Choices, 2)
	assert.Equal(t, "draft", fields[2].Choices[0].Value)

	assert.Equal(t, "Views", fields[3].Name)
	assert.Equal(t, "number", fields[3].Type)
	assert.Equal(t, 42, fields[3].Value)
}

func TestExtractFieldsWithErrors(t *testing.T) {
	instance := &articleForm{}
	ve := &burrow.ValidationError{
		Errors: []burrow.FieldError{
			{Field: "title", Message: "title is required"},
			{Field: "title", Message: "title must be at least 3"},
		},
	}

	fields := extractFields(noCtx, nil, instance, ve, nil, nil)

	require.Len(t, fields, 4)
	assert.Equal(t, []string{"title is required", "title must be at least 3"}, fields[0].Errors)
	assert.Empty(t, fields[1].Errors)
}

func TestExtractFieldsWithDynamicChoices(t *testing.T) {
	instance := &articleForm{Views: 5}
	choices := map[string][]Choice{
		"Views": {
			{Value: "1", Label: "One"},
			{Value: "2", Label: "Two"},
		},
	}

	fields := extractFields(noCtx, nil, instance, nil, choices, nil)

	// Views should be overridden to select with dynamic choices.
	viewsField := fields[3]
	assert.Equal(t, "select", viewsField.Type)
	assert.Len(t, viewsField.Choices, 2)
	assert.Equal(t, "One", viewsField.Choices[0].Label)
}

type embeddedBase struct {
	Meta string
}

type formWithEmbedded struct {
	embeddedBase
	Name string `form:"name"`
}

func TestExtractFieldsSkipsEmbedded(t *testing.T) {
	instance := &formWithEmbedded{Name: "test"}
	fields := extractFields(noCtx, nil, instance, nil, nil, nil)
	require.Len(t, fields, 1)
	assert.Equal(t, "Name", fields[0].Name)
}

type formWithUnexported struct {
	Name    string `form:"name"`
	private string //nolint:unused // testing unexported fields are skipped
}

func TestExtractFieldsSkipsUnexported(t *testing.T) {
	instance := &formWithUnexported{Name: "test"}
	fields := extractFields(noCtx, nil, instance, nil, nil, nil)
	require.Len(t, fields, 1)
	assert.Equal(t, "Name", fields[0].Name)
}

func TestExtractFieldsWithExclude(t *testing.T) {
	instance := &articleForm{
		Title:   "Hello",
		Content: "World",
		Status:  "draft",
		Views:   42,
	}

	exclude := map[string]struct{}{"Title": {}, "Views": {}}
	fields := extractFields(noCtx, nil, instance, nil, nil, exclude)

	require.Len(t, fields, 2) // Only Content and Status remain
	assert.Equal(t, "Content", fields[0].Name)
	assert.Equal(t, "Status", fields[1].Name)
}

func TestExtractFieldsWithNilExclude(t *testing.T) {
	instance := &articleForm{
		Title: "Hello",
	}

	// nil exclude should return all fields (same as before).
	fields := extractFields(noCtx, nil, instance, nil, nil, nil)
	require.Len(t, fields, 4)
}

func TestFieldErrorsHelper(t *testing.T) {
	ve := &burrow.ValidationError{
		Errors: []burrow.FieldError{
			{Field: "email", Message: "invalid email"},
			{Field: "name", Message: "required"},
			{Field: "email", Message: "too long"},
		},
	}

	errs := fieldErrors(ve, "email")
	assert.Equal(t, []string{"invalid email", "too long"}, errs)

	errs = fieldErrors(ve, "name")
	assert.Equal(t, []string{"required"}, errs)

	errs = fieldErrors(ve, "missing")
	assert.Nil(t, errs)
}
