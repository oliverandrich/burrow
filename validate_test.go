package burrow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateStructWithoutTags(t *testing.T) {
	v := struct {
		Name string
	}{Name: "alice"}

	err := Validate(v)
	assert.NoError(t, err)
}

func TestValidateStructRequiredEmpty(t *testing.T) {
	v := struct {
		Email string `validate:"required"`
	}{}

	err := Validate(v)

	require.Error(t, err)

	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	require.Len(t, ve.Errors, 1)
	assert.Equal(t, "Email", ve.Errors[0].Field)
	assert.Equal(t, "required", ve.Errors[0].Tag)
}

func TestValidateErrorsAs(t *testing.T) {
	v := struct {
		Email string `validate:"required,email"`
	}{}

	err := Validate(v)

	var ve *ValidationError
	assert.ErrorAs(t, err, &ve)
}

func TestValidateFieldNameFromFormTag(t *testing.T) {
	v := struct {
		EmailAddr string `form:"email" validate:"required"`
	}{}

	err := Validate(v)

	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "email", ve.Errors[0].Field)
}

func TestValidateFieldNameFromJSONTag(t *testing.T) {
	v := struct {
		EmailAddr string `json:"email_address" validate:"required"`
	}{}

	err := Validate(v)

	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "email_address", ve.Errors[0].Field)
}

func TestValidateFieldNameFormTagPrecedence(t *testing.T) {
	v := struct {
		EmailAddr string `form:"email" json:"email_address" validate:"required"`
	}{}

	err := Validate(v)

	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "email", ve.Errors[0].Field)
}

func TestValidateHasField(t *testing.T) {
	v := struct {
		Email string `form:"email" validate:"required"`
		Name  string `form:"name" validate:"required"`
	}{}

	err := Validate(v)

	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	assert.True(t, ve.HasField("email"))
	assert.True(t, ve.HasField("name"))
	assert.False(t, ve.HasField("phone"))
}

func TestValidateMultipleErrors(t *testing.T) {
	v := struct {
		Email string `form:"email" validate:"required"`
		Name  string `form:"name" validate:"required"`
		Age   int    `form:"age" validate:"required,gte=1"`
	}{}

	err := Validate(v)

	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Len(t, ve.Errors, 3)
}

func TestValidateErrorMessage(t *testing.T) {
	v := struct {
		Email string `form:"email" validate:"required"`
	}{}

	err := Validate(v)

	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Contains(t, ve.Error(), "validation failed")
	assert.Contains(t, ve.Error(), "email")
}

func TestValidatePassingStruct(t *testing.T) {
	v := struct {
		Email string `form:"email" validate:"required,email"`
		Name  string `form:"name" validate:"required"`
	}{
		Email: "alice@example.com",
		Name:  "Alice",
	}

	err := Validate(v)
	assert.NoError(t, err)
}

func TestValidateNonStructReturnsNil(t *testing.T) {
	s := "just a string"
	err := Validate(s)
	assert.NoError(t, err)
}

func TestValidateFieldErrorParam(t *testing.T) {
	v := struct {
		Name string `validate:"min=3"`
	}{Name: "ab"}

	err := Validate(v)

	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	require.Len(t, ve.Errors, 1)
	assert.Equal(t, "min", ve.Errors[0].Tag)
	assert.Equal(t, "3", ve.Errors[0].Param)
}

func TestValidateFieldErrorParamEmpty(t *testing.T) {
	v := struct {
		Name string `validate:"required"`
	}{}

	err := Validate(v)

	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	require.Len(t, ve.Errors, 1)
	assert.Equal(t, "required", ve.Errors[0].Tag)
	assert.Empty(t, ve.Errors[0].Param)
}

func TestValidationErrorTranslate(t *testing.T) {
	ve := &ValidationError{
		Errors: []FieldError{
			{Field: "Email", Tag: "required", Message: "Email is required"},
			{Field: "Age", Tag: "min", Param: "18", Message: "Age must be at least 18"},
		},
	}

	// Mock translation function that adds a prefix.
	mockTranslate := func(_ context.Context, key string, data map[string]any) string {
		if key == "validation-required" {
			return data["Field"].(string) + " ist erforderlich"
		}
		if key == "validation-min" {
			return data["Field"].(string) + " muss mindestens " + data["Param"].(string) + " sein"
		}
		return key
	}

	ve.Translate(context.Background(), mockTranslate)

	assert.Equal(t, "Email ist erforderlich", ve.Errors[0].Message)
	assert.Equal(t, "Age muss mindestens 18 sein", ve.Errors[1].Message)
}

func TestValidationErrorTranslatePreservesUnknownTags(t *testing.T) {
	ve := &ValidationError{
		Errors: []FieldError{
			{Field: "Code", Tag: "custom_tag", Message: "Code failed custom_tag validation"},
		},
	}

	// Translation function returns the key for unknown tags.
	mockTranslate := func(_ context.Context, key string, _ map[string]any) string {
		return key
	}

	ve.Translate(context.Background(), mockTranslate)
	assert.Equal(t, "Code failed custom_tag validation", ve.Errors[0].Message)
}
