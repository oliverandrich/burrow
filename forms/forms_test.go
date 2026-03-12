package forms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type loginForm struct {
	Email    string `form:"email" verbose_name:"Email" widget:"email" validate:"required,email"`
	Password string `form:"password" verbose_name:"Password" validate:"required,min=8"`
}

func TestNewFormEmpty(t *testing.T) {
	f := New[loginForm]()

	assert.False(t, f.IsValid())
	assert.NotNil(t, f.Instance())
	assert.Empty(t, f.Instance().Email)
	assert.Empty(t, f.Instance().Password)
}

func TestNewFormWithInitial(t *testing.T) {
	f := New(WithInitial[loginForm](map[string]any{
		"email": "test@example.com",
	}))

	assert.Equal(t, "test@example.com", f.Instance().Email)
	assert.Empty(t, f.Instance().Password)
}

func TestFromModelNil(t *testing.T) {
	f := FromModel[loginForm](nil)

	assert.Empty(t, f.Instance().Email)
}

func TestFromModelWithInstance(t *testing.T) {
	existing := &loginForm{
		Email:    "existing@example.com",
		Password: "secret123",
	}
	f := FromModel(existing)

	assert.Equal(t, "existing@example.com", f.Instance().Email)
	assert.Equal(t, "secret123", f.Instance().Password)
}

func TestBindValid(t *testing.T) {
	f := New[loginForm]()
	r := postRequest(url.Values{
		"email":    {"user@example.com"},
		"password": {"longpassword"},
	})

	valid := f.Bind(r)

	assert.True(t, valid)
	assert.True(t, f.IsValid())
	assert.Equal(t, "user@example.com", f.Instance().Email)
	assert.Equal(t, "longpassword", f.Instance().Password)
	assert.Nil(t, f.Errors())
}

func TestBindInvalid(t *testing.T) {
	f := New[loginForm]()
	r := postRequest(url.Values{
		"email":    {"not-an-email"},
		"password": {"short"},
	})

	valid := f.Bind(r)

	assert.False(t, valid)
	assert.False(t, f.IsValid())
	require.NotNil(t, f.Errors())
	assert.True(t, f.Errors().HasField("email"))
	assert.True(t, f.Errors().HasField("password"))
}

func TestBindFieldErrors(t *testing.T) {
	f := New[loginForm]()
	r := postRequest(url.Values{
		"email":    {""},
		"password": {""},
	})

	f.Bind(r)
	fields := f.Fields()

	// Both fields should have errors.
	emailField, ok := f.Field("Email")
	require.True(t, ok)
	assert.NotEmpty(t, emailField.Errors)

	passwordField, ok := f.Field("Password")
	require.True(t, ok)
	assert.NotEmpty(t, passwordField.Errors)

	assert.Len(t, fields, 2)
}

func TestFieldsOrder(t *testing.T) {
	f := New[loginForm]()

	fields := f.Fields()

	require.Len(t, fields, 2)
	assert.Equal(t, "Email", fields[0].Name)
	assert.Equal(t, "Password", fields[1].Name)
}

func TestFieldLookup(t *testing.T) {
	f := New[loginForm]()

	_, ok := f.Field("Email")
	assert.True(t, ok)

	_, ok = f.Field("Nonexistent")
	assert.False(t, ok)
}

type cleanableForm struct {
	Start string `form:"start" validate:"required"`
	End   string `form:"end" validate:"required"`
}

func (f *cleanableForm) Clean() error {
	if f.Start != "" && f.End != "" && f.Start > f.End {
		return &burrow.ValidationError{
			Errors: []burrow.FieldError{
				{Field: "end", Message: "end must be after start"},
			},
		}
	}
	return nil
}

func TestBindWithClean(t *testing.T) {
	f := New[cleanableForm]()
	r := postRequest(url.Values{
		"start": {"2024-02-01"},
		"end":   {"2024-01-01"},
	})

	valid := f.Bind(r)

	assert.False(t, valid)
	require.NotNil(t, f.Errors())
	assert.True(t, f.Errors().HasField("end"))
}

func TestBindWithCleanValid(t *testing.T) {
	f := New[cleanableForm]()
	r := postRequest(url.Values{
		"start": {"2024-01-01"},
		"end":   {"2024-02-01"},
	})

	valid := f.Bind(r)

	assert.True(t, valid)
}

type choiceProviderForm struct {
	Category string `form:"category" verbose_name:"Category"`
}

func (f *choiceProviderForm) FieldChoices(_ context.Context, field string) ([]Choice, error) {
	if field == "Category" {
		return []Choice{
			{Value: "tech", Label: "Technology"},
			{Value: "sci", Label: "Science"},
		}, nil
	}
	return nil, nil
}

func TestBindWithChoiceProvider(t *testing.T) {
	f := New[choiceProviderForm]()
	r := postRequest(url.Values{
		"category": {"tech"},
	})

	valid := f.Bind(r)

	assert.True(t, valid)
	assert.Equal(t, "tech", f.Instance().Category)

	// Fields should include dynamic choices.
	fields := f.Fields()
	require.Len(t, fields, 1)
	assert.Equal(t, "select", fields[0].Type)
	assert.Len(t, fields[0].Choices, 2)
}

func TestWithChoicesOption(t *testing.T) {
	choices := []Choice{
		{Value: "a", Label: "Alpha"},
		{Value: "b", Label: "Beta"},
	}
	f := New(WithChoices[choiceProviderForm]("Category", choices))

	fields := f.Fields()
	require.Len(t, fields, 1)
	assert.Equal(t, "select", fields[0].Type)
	assert.Equal(t, choices, fields[0].Choices)
}

func TestWithChoicesFuncOption(t *testing.T) {
	fn := func(_ context.Context) ([]Choice, error) {
		return []Choice{{Value: "x", Label: "X"}}, nil
	}
	f := New(WithChoicesFunc[choiceProviderForm]("Category", fn))

	r := postRequest(url.Values{"category": {"x"}})
	valid := f.Bind(r)

	assert.True(t, valid)
	fields := f.Fields()
	require.Len(t, fields, 1)
	assert.Len(t, fields[0].Choices, 1)
}

func TestNonFieldErrors(t *testing.T) {
	f := New[cleanableForm]()
	r := postRequest(url.Values{
		"start": {"2024-02-01"},
		"end":   {"2024-01-01"},
	})

	f.Bind(r)

	// The clean error is a field error, not a non-field error.
	assert.Empty(t, f.NonFieldErrors())
}

type nonFieldCleanForm struct {
	A string `form:"a" validate:"required"`
}

func (f *nonFieldCleanForm) Clean() error {
	return &burrow.ValidationError{
		Errors: []burrow.FieldError{
			{Field: "", Message: "something is globally wrong"},
		},
	}
}

func TestNonFieldErrorsFromClean(t *testing.T) {
	f := New[nonFieldCleanForm]()
	r := postRequest(url.Values{"a": {"hello"}})

	f.Bind(r)

	nfe := f.NonFieldErrors()
	require.Len(t, nfe, 1)
	assert.Equal(t, "something is globally wrong", nfe[0])
}

func TestFromModelPreservesValues(t *testing.T) {
	existing := &loginForm{
		Email:    "old@example.com",
		Password: "oldpassword",
	}
	f := FromModel(existing)

	// Before bind, instance should have model values.
	assert.Equal(t, "old@example.com", f.Instance().Email)

	// After bind, instance should have form values.
	r := postRequest(url.Values{
		"email":    {"new@example.com"},
		"password": {"newpassword123"},
	})
	valid := f.Bind(r)
	assert.True(t, valid)
	assert.Equal(t, "new@example.com", f.Instance().Email)
}

func TestWithExcludeOption(t *testing.T) {
	f := New(WithExclude[loginForm]("Password"))

	fields := f.Fields()
	require.Len(t, fields, 1)
	assert.Equal(t, "Email", fields[0].Name)
}

func TestWithExcludeMultipleFields(t *testing.T) {
	f := New(WithExclude[loginForm]("Email", "Password"))

	fields := f.Fields()
	assert.Empty(t, fields)
}

func TestWithExcludeFromModel(t *testing.T) {
	existing := &loginForm{
		Email:    "test@example.com",
		Password: "secret",
	}
	f := FromModel(existing, WithExclude[loginForm]("Password"))

	fields := f.Fields()
	require.Len(t, fields, 1)
	assert.Equal(t, "Email", fields[0].Name)
	assert.Equal(t, "test@example.com", fields[0].Value)

	// Instance still has all values — exclude only affects Fields().
	assert.Equal(t, "secret", f.Instance().Password)
}

func TestFieldsAutoTranslatesErrors(t *testing.T) {
	// Create a mock translate function that simulates i18n.TData.
	mockTranslate := func(_ context.Context, key string, data map[string]any) string {
		if key == "validation-required" {
			return data["Field"].(string) + " ist erforderlich"
		}
		if key == "validation-email" {
			return data["Field"].(string) + " muss eine gültige E-Mail-Adresse sein"
		}
		return key
	}

	f := New(WithTranslateFunc[loginForm](mockTranslate))
	r := postRequest(url.Values{
		"email":    {""},
		"password": {""},
	})

	f.Bind(r)
	fields := f.Fields()

	emailField := fields[0]
	require.NotEmpty(t, emailField.Errors)
	assert.Equal(t, "email ist erforderlich", emailField.Errors[0])

	passwordField := fields[1]
	require.NotEmpty(t, passwordField.Errors)
	assert.Equal(t, "password ist erforderlich", passwordField.Errors[0])
}

func TestFieldsWithoutTranslateFuncUsesEnglishFallback(t *testing.T) {
	f := New[loginForm]()
	r := postRequest(url.Values{
		"email":    {""},
		"password": {""},
	})

	f.Bind(r)
	fields := f.Fields()

	emailField := fields[0]
	require.NotEmpty(t, emailField.Errors)
	assert.Equal(t, "email is required", emailField.Errors[0])
}

// postRequest creates a form POST request for testing.
func postRequest(values url.Values) *http.Request {
	body := values.Encode()
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}
