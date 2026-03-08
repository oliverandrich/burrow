package i18n

import (
	"context"
	"testing"
	"testing/fstest"

	"codeberg.org/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func TestTranslateValidationErrorsEnglish(t *testing.T) {
	app := configuredApp(t)
	ctx := app.WithLocale(context.Background(), "en")

	ve := &burrow.ValidationError{
		Errors: []burrow.FieldError{
			{Field: "Email", Tag: "required", Message: "Email is required"},
		},
	}

	TranslateValidationErrors(ctx, ve)
	assert.Equal(t, "Email is required", ve.Errors[0].Message)
}

func TestTranslateValidationErrorsGerman(t *testing.T) {
	app := configuredApp(t)
	ctx := app.WithLocale(context.Background(), "de")

	ve := &burrow.ValidationError{
		Errors: []burrow.FieldError{
			{Field: "Email", Tag: "required", Message: "Email is required"},
		},
	}

	TranslateValidationErrors(ctx, ve)
	assert.Equal(t, "Email ist erforderlich", ve.Errors[0].Message)
}

func TestTranslateValidationErrorsUnknownTag(t *testing.T) {
	app := configuredApp(t)
	ctx := app.WithLocale(context.Background(), "en")

	ve := &burrow.ValidationError{
		Errors: []burrow.FieldError{
			{Field: "Code", Tag: "custom_tag", Message: "Code failed custom_tag validation"},
		},
	}

	TranslateValidationErrors(ctx, ve)
	assert.Equal(t, "Code failed custom_tag validation", ve.Errors[0].Message)
}

func TestTranslateValidationErrorsWithoutLocalizer(t *testing.T) {
	ctx := context.Background()

	ve := &burrow.ValidationError{
		Errors: []burrow.FieldError{
			{Field: "Email", Tag: "required", Message: "Email is required"},
		},
	}

	TranslateValidationErrors(ctx, ve)
	assert.Equal(t, "Email is required", ve.Errors[0].Message)
}

func TestTranslateValidationErrorsAllTags(t *testing.T) {
	app := configuredApp(t)
	ctx := app.WithLocale(context.Background(), "de")

	ve := &burrow.ValidationError{
		Errors: []burrow.FieldError{
			{Field: "Name", Tag: "required", Message: "original"},
			{Field: "Email", Tag: "email", Message: "original"},
			{Field: "Age", Tag: "min", Param: "18", Message: "original"},
			{Field: "Score", Tag: "max", Param: "100", Message: "original"},
			{Field: "Code", Tag: "len", Param: "6", Message: "original"},
			{Field: "Rating", Tag: "gte", Param: "1", Message: "original"},
			{Field: "Price", Tag: "lte", Param: "999", Message: "original"},
			{Field: "Website", Tag: "url", Message: "original"},
		},
	}

	TranslateValidationErrors(ctx, ve)

	assert.Equal(t, "Name ist erforderlich", ve.Errors[0].Message)
	assert.Equal(t, "Email muss eine gültige E-Mail-Adresse sein", ve.Errors[1].Message)
	assert.Equal(t, "Age muss mindestens 18 sein", ve.Errors[2].Message)
	assert.Equal(t, "Score darf höchstens 100 sein", ve.Errors[3].Message)
	assert.Equal(t, "Code muss genau 6 Zeichen lang sein", ve.Errors[4].Message)
	assert.Equal(t, "Rating muss größer oder gleich 1 sein", ve.Errors[5].Message)
	assert.Equal(t, "Price muss kleiner oder gleich 999 sein", ve.Errors[6].Message)
	assert.Equal(t, "Website muss eine gültige URL sein", ve.Errors[7].Message)
}

func TestTranslateValidationErrorsEndToEnd(t *testing.T) {
	app := configuredApp(t)
	ctx := app.WithLocale(context.Background(), "de")

	v := struct {
		Name  string `validate:"required"`
		Email string `validate:"required,email"`
		Age   int    `validate:"min=18"`
	}{
		Email: "not-an-email",
	}

	err := burrow.Validate(v)
	require.Error(t, err)

	var ve *burrow.ValidationError
	require.ErrorAs(t, err, &ve)

	TranslateValidationErrors(ctx, ve)

	assert.Equal(t, "Name ist erforderlich", ve.Errors[0].Message)
	assert.Equal(t, "Email muss eine gültige E-Mail-Adresse sein", ve.Errors[1].Message)
	assert.Equal(t, "Age muss mindestens 18 sein", ve.Errors[2].Message)
}

func TestTranslateValidationErrorsAppOverride(t *testing.T) {
	mock := &mockTranslationApp{
		fs: fstest.MapFS{
			"translations/active.de.toml": &fstest.MapFile{
				Data: []byte("validation-required = \"{{.Field}} wird benötigt\"\n"),
			},
		},
	}

	registry := burrow.NewRegistry()
	registry.Add(mock)

	app := New()
	require.NoError(t, app.Register(&burrow.AppConfig{Registry: registry}))

	cmd := &cli.Command{
		Name:  "test",
		Flags: app.Flags(nil),
		Action: func(_ context.Context, cmd *cli.Command) error {
			return app.Configure(cmd)
		},
	}
	err := cmd.Run(t.Context(), []string{"test"})
	require.NoError(t, err)

	ctx := app.WithLocale(context.Background(), "de")

	ve := &burrow.ValidationError{
		Errors: []burrow.FieldError{
			{Field: "Name", Tag: "required", Message: "Name is required"},
		},
	}

	TranslateValidationErrors(ctx, ve)
	assert.Equal(t, "Name wird benötigt", ve.Errors[0].Message)
}
