package forms

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/i18n"
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

	fields := extractFields(noCtx, instance, nil, nil, nil, nil)

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

	fields := extractFields(noCtx, instance, ve, nil, nil, nil)

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

	fields := extractFields(noCtx, instance, nil, choices, nil, nil)

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
	fields := extractFields(noCtx, instance, nil, nil, nil, nil)
	require.Len(t, fields, 1)
	assert.Equal(t, "Name", fields[0].Name)
}

type formWithUnexported struct {
	Name    string `form:"name"`
	private string //nolint:unused // testing unexported fields are skipped
}

func TestExtractFieldsSkipsUnexported(t *testing.T) {
	instance := &formWithUnexported{Name: "test"}
	fields := extractFields(noCtx, instance, nil, nil, nil, nil)
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
	fields := extractFields(noCtx, instance, nil, nil, exclude, nil)

	require.Len(t, fields, 2) // Only Content and Status remain
	assert.Equal(t, "Content", fields[0].Name)
	assert.Equal(t, "Status", fields[1].Name)
}

func TestExtractFieldsWithNilExclude(t *testing.T) {
	instance := &articleForm{
		Title: "Hello",
	}

	// nil exclude should return all fields (same as before).
	fields := extractFields(noCtx, instance, nil, nil, nil, nil)
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

	errs := fieldErrors(noCtx, ve, "email")
	assert.Equal(t, []string{"invalid email", "too long"}, errs)

	errs = fieldErrors(noCtx, ve, "name")
	assert.Equal(t, []string{"required"}, errs)

	errs = fieldErrors(noCtx, ve, "missing")
	assert.Nil(t, errs)
}

type formWithPointer struct {
	Email *string `form:"email" verbose_name:"Email"`
	Name  string  `form:"name" verbose_name:"Name"`
}

func TestExtractFieldsDereferencesPointer(t *testing.T) {
	email := "test@example.com"
	instance := &formWithPointer{Email: &email, Name: "Alice"}

	fields := extractFields(noCtx, instance, nil, nil, nil, nil)

	require.Len(t, fields, 2)
	assert.Equal(t, "test@example.com", fields[0].Value)
	assert.Equal(t, "Alice", fields[1].Value)
}

func TestExtractFieldsNilPointerValue(t *testing.T) {
	instance := &formWithPointer{Email: nil, Name: "Bob"}

	fields := extractFields(noCtx, instance, nil, nil, nil, nil)

	require.Len(t, fields, 2)
	assert.Empty(t, fields[0].Value, "nil *string should produce zero value")
	assert.Equal(t, "Bob", fields[1].Value)
}

// labelI18nFS provides a minimal translation bundle covering the labels and
// choice values used by [TestExtractFieldsTranslatesLabels].
var labelI18nFS = fstest.MapFS{ //nolint:gochecknoglobals // test fixture
	"translations/active.en.toml": &fstest.MapFile{
		Data: []byte(`Title = "Title"
Content = "Content"
Status = "Status"
draft = "Draft"
published = "Published"
`),
	},
	"translations/active.de.toml": &fstest.MapFile{
		Data: []byte(`Title = "Titel"
Content = "Inhalt"
Status = "Status"
draft = "Entwurf"
published = "Veröffentlicht"
Item = "Eintrag"
`),
	},
}

// findField returns the BoundField with the given Go struct field Name.
// Tests prefer this over positional indexing so they don't break when
// articleForm gains a new field.
func findField(t *testing.T, fields []BoundField, name string) BoundField {
	t.Helper()
	for _, bf := range fields {
		if bf.Name == name {
			return bf
		}
	}
	t.Fatalf("field %q not found in %d extracted fields", name, len(fields))
	return BoundField{}
}

func TestExtractFieldsTranslatesLabels(t *testing.T) {
	bundle, err := i18n.NewTestBundle("en", labelI18nFS)
	require.NoError(t, err)

	instance := &articleForm{Title: "x", Content: "y", Status: "draft"}

	t.Run("German locale translates BoundField labels and Choice labels", func(t *testing.T) {
		ctx := bundle.WithLocale(context.Background(), "de")
		fields := extractFields(ctx, instance, nil, nil, nil, nil)

		assert.Equal(t, "Titel", findField(t, fields, "Title").Label, "verbose tag must be piped through i18n.T")
		assert.Equal(t, "Inhalt", findField(t, fields, "Content").Label)

		status := findField(t, fields, "Status")
		require.Len(t, status.Choices, 2)
		assert.Equal(t, "Entwurf", status.Choices[0].Label, "tag-based Choice labels must translate too")
		assert.Equal(t, "Veröffentlicht", status.Choices[1].Label)
	})

	t.Run("Dynamic choices (WithChoices/WithChoicesFunc) translate too", func(t *testing.T) {
		ctx := bundle.WithLocale(context.Background(), "de")
		dynamic := map[string][]Choice{
			"Views": {{Value: "1", Label: "Item"}},
		}
		fields := extractFields(ctx, instance, nil, dynamic, nil, nil)

		views := findField(t, fields, "Views")
		require.Len(t, views.Choices, 1)
		assert.Equal(t, "Eintrag", views.Choices[0].Label, "dynamic Choice labels must go through the same translation path as static ones")
	})

	t.Run("Without a localizer in context, labels fall back to the raw Label", func(t *testing.T) {
		fields := extractFields(context.Background(), instance, nil, nil, nil, nil)

		assert.Equal(t, "Title", findField(t, fields, "Title").Label, "no localizer must yield the raw verbose tag value")
		assert.Equal(t, "draft", findField(t, fields, "Status").Choices[0].Label)
	})
}

// TestExtractFieldsDoesNotMutateChoiceSource locks in that extractFields
// does not mutate the caller's Choice slice in place when translating
// labels — otherwise a second render would read the already-translated
// string as the message ID, and any package-level slice handed to
// WithChoices would bleed locale state between requests.
func TestExtractFieldsDoesNotMutateChoiceSource(t *testing.T) {
	bundle, err := i18n.NewTestBundle("en", labelI18nFS)
	require.NoError(t, err)
	ctx := bundle.WithLocale(context.Background(), "de")

	source := []Choice{{Value: "1", Label: "Item"}}
	dynamic := map[string][]Choice{"Views": source}
	instance := &articleForm{}

	first := extractFields(ctx, instance, nil, dynamic, nil, nil)
	require.Equal(t, "Eintrag", findField(t, first, "Views").Choices[0].Label, "sanity: first call returns the translation")

	assert.Equal(t, "Item", source[0].Label, "caller's slice must not be mutated by extractFields")

	second := extractFields(ctx, instance, nil, dynamic, nil, nil)
	assert.Equal(t, "Eintrag", findField(t, second, "Views").Choices[0].Label, "second call must translate from the original Label, not from the first call's output")
}
