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

func TestFieldErrorMessageNewTags(t *testing.T) {
	tests := []struct {
		tag      string
		validate string
		value    any
		wantSub  string // substring that must appear in the message
	}{
		{tag: "http_url", validate: "http_url", value: "not-a-url", wantSub: "valid HTTP URL"},
		{tag: "uri", validate: "uri", value: "://bad", wantSub: "valid URI"},
		{tag: "alpha", validate: "alpha", value: "abc123", wantSub: "letters only"},
		{tag: "alphanum", validate: "alphanum", value: "abc!@#", wantSub: "letters and numbers only"},
		{tag: "alphaunicode", validate: "alphaunicode", value: "abc123", wantSub: "letters only"},
		{tag: "alphanumunicode", validate: "alphanumunicode", value: "abc!@#", wantSub: "letters and numbers only"},
		{tag: "numeric", validate: "numeric", value: "abc", wantSub: "valid number"},
		{tag: "number", validate: "number", value: "abc", wantSub: "valid number"},
		{tag: "boolean", validate: "boolean", value: "maybe", wantSub: "valid boolean"},
		{tag: "uuid", validate: "uuid", value: "not-a-uuid", wantSub: "valid UUID"},
		{tag: "uuid4", validate: "uuid4", value: "not-a-uuid", wantSub: "valid UUID"},
		{tag: "unique", validate: "unique", value: []string{"a", "a"}, wantSub: "unique values"},
		{tag: "ip", validate: "ip", value: "not-an-ip", wantSub: "valid IP address"},
		{tag: "ipv4", validate: "ipv4", value: "not-an-ip", wantSub: "valid IPv4 address"},
		{tag: "contains", validate: "contains=foo", value: "bar", wantSub: "must contain"},
		{tag: "startswith", validate: "startswith=foo", value: "bar", wantSub: "must start with"},
		{tag: "endswith", validate: "endswith=foo", value: "bar", wantSub: "must end with"},
		{tag: "lowercase", validate: "lowercase", value: "ABC", wantSub: "lowercase"},
		{tag: "uppercase", validate: "uppercase", value: "abc", wantSub: "uppercase"},
		{tag: "gt", validate: "gt=5", value: 3, wantSub: "greater than"},
		{tag: "lt", validate: "lt=5", value: 10, wantSub: "less than"},
		{tag: "eq", validate: "eq=5", value: 3, wantSub: "equal to"},
		{tag: "ne", validate: "ne=5", value: 5, wantSub: "must not equal"},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			// Build a dynamic struct with the validate tag.
			err := structValidator.Var(tt.value, tt.validate)
			require.Error(t, err)

			ve := toValidationError(err)
			var valErr *ValidationError
			require.ErrorAs(t, ve, &valErr)
			require.Len(t, valErr.Errors, 1)

			msg := valErr.Errors[0].Message
			assert.Contains(t, msg, tt.wantSub, "tag %q: message %q should contain %q", tt.tag, msg, tt.wantSub)
			// Must NOT be the generic fallback.
			assert.NotContains(t, msg, "failed", "tag %q: message %q should not be the generic fallback", tt.tag, msg)
		})
	}
}

func TestFieldErrorMessageOneofTag(t *testing.T) {
	type oneofForm struct {
		Color string `validate:"oneof=red green blue"`
	}
	err := Validate(oneofForm{Color: "yellow"})
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	require.Len(t, ve.Errors, 1)
	assert.Contains(t, ve.Errors[0].Message, "must be one of")
}

func TestFieldErrorMessageEqfieldTag(t *testing.T) {
	type passwordForm struct {
		Password string `validate:"required"`
		Confirm  string `validate:"required,eqfield=Password"`
	}
	err := Validate(passwordForm{Password: "abc", Confirm: "xyz"})
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	// Find the eqfield error.
	var found bool
	for _, fe := range ve.Errors {
		if fe.Tag == "eqfield" {
			assert.Contains(t, fe.Message, "must match")
			found = true
		}
	}
	assert.True(t, found)
}

func TestFieldErrorMessageNefieldTag(t *testing.T) {
	type diffForm struct {
		A string `validate:"required"`
		B string `validate:"required,nefield=A"`
	}
	err := Validate(diffForm{A: "same", B: "same"})
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	var found bool
	for _, fe := range ve.Errors {
		if fe.Tag == "nefield" {
			assert.Contains(t, fe.Message, "must differ")
			found = true
		}
	}
	assert.True(t, found)
}

func TestFieldErrorMessageDatetimeTag(t *testing.T) {
	type dateForm struct {
		Date string `validate:"datetime=2006-01-02"`
	}
	err := Validate(dateForm{Date: "not-a-date"})
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	require.Len(t, ve.Errors, 1)
	assert.Contains(t, ve.Errors[0].Message, "valid date/time")
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
