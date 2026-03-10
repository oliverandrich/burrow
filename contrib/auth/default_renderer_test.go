package auth

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/csrf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed templates/*.html
var testRendererTemplateFS embed.FS

// Compile-time interface assertion.
var _ Renderer = (*defaultRenderer)(nil)

// rendererTestExecutor creates a TemplateExecutor for testing with stub functions.
func rendererTestExecutor() burrow.TemplateExecutor {
	funcMap := template.FuncMap{
		"t":                    func(key string) string { return key },
		"csrfToken":            func() string { return "test-csrf-token" },
		"staticURL":            func(name string) string { return "/static/" + name },
		"authLogo":             func() template.HTML { return "" },
		"currentUser":          func() *User { return nil },
		"isAuthenticated":      func() bool { return false },
		"isAdminEditSelf":      func() bool { return false },
		"isAdminEditLastAdmin": func() bool { return false },
		"lang":                 func() string { return "en" },
		"itoa":                 func(id int64) string { return template.HTMLEscapeString(fmt.Sprintf("%d", id)) },
		"credName":             credName,
		"emailValue":           func(user *User) string { return "" },
		"deref": func(s *string) string {
			if s != nil {
				return *s
			}
			return ""
		},
	}

	tmpl := template.Must(template.New("").Funcs(funcMap).Parse(`{{ define "bootstrap/theme_script" }}{{ end }}`))
	template.Must(tmpl.ParseFS(testRendererTemplateFS, "templates/*.html"))

	return func(r *http.Request, name string, data map[string]any) (template.HTML, error) {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
			return "", err
		}
		return template.HTML(buf.String()), nil //nolint:gosec // test
	}
}

// withRendererTestExecutor sets up a request with a test template executor in context.
func withRendererTestExecutor(req *http.Request) *http.Request {
	ctx := burrow.WithTemplateExecutor(req.Context(), rendererTestExecutor())
	return req.WithContext(ctx)
}

func TestDefaultRendererLoginPage(t *testing.T) {
	r := DefaultRenderer()
	req := withRendererTestExecutor(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/login", nil))
	rec := httptest.NewRecorder()

	err := r.LoginPage(rec, req, "/dashboard")

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "login-button")
	assert.NotContains(t, body, "card shadow-sm", "login page should not have a card frame")
}

func TestDefaultRendererRegisterPage(t *testing.T) {
	r := DefaultRenderer()
	req := withRendererTestExecutor(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/register", nil))
	rec := httptest.NewRecorder()

	err := r.RegisterPage(rec, req, false, false, "", "")

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "register-username-label")
	assert.Contains(t, body, "card shadow-sm", "register page should be wrapped in a card")
}

func TestDefaultRendererRegisterPageEmailMode(t *testing.T) {
	r := DefaultRenderer()
	req := withRendererTestExecutor(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/register", nil))
	rec := httptest.NewRecorder()

	err := r.RegisterPage(rec, req, true, false, "", "")

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "register-email-label")
	assert.NotContains(t, rec.Body.String(), "register-username-label")
}

func TestDefaultRendererCredentialsPage(t *testing.T) {
	r := DefaultRenderer()
	creds := []Credential{
		{ID: 1, Name: "My Passkey"},
	}
	req := withRendererTestExecutor(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/credentials", nil))
	rec := httptest.NewRecorder()

	err := r.CredentialsPage(rec, req, creds)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "My Passkey")
	assert.Contains(t, body, "card shadow-sm", "credentials page should be wrapped in a card")
}

func TestDefaultRendererRecoveryPage(t *testing.T) {
	r := DefaultRenderer()
	req := withRendererTestExecutor(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/recovery", nil))
	rec := httptest.NewRecorder()

	err := r.RecoveryPage(rec, req, "/dashboard")

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "recovery-description")
	assert.NotContains(t, body, "card-title", "recovery page should not have a card title")
	assert.Contains(t, body, "card shadow-sm", "recovery page should be wrapped in a card")
}

func TestDefaultRendererRecoveryCodesPage(t *testing.T) {
	r := DefaultRenderer()
	codes := []string{"aaaa-bbbb-cccc", "dddd-eeee-ffff"}
	req := withRendererTestExecutor(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/recovery-codes", nil))
	rec := httptest.NewRecorder()

	err := r.RecoveryCodesPage(rec, req, codes)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "recovery-codes-title")
	assert.Contains(t, body, "aaaa-bbbb-cccc")
	assert.Contains(t, body, "dddd-eeee-ffff")
	assert.Contains(t, body, "card shadow-sm", "recovery codes page should be wrapped in a card")
	assert.Contains(t, body, "/auth/recovery-codes/ack")
}

func TestDefaultRendererVerifyPendingPage(t *testing.T) {
	r := DefaultRenderer()
	req := withRendererTestExecutor(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/verify-pending", nil))
	rec := httptest.NewRecorder()

	err := r.VerifyPendingPage(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "verify-pending-title")
	assert.Contains(t, body, "card shadow-sm", "verify pending page should be wrapped in a card")
}

func TestDefaultRendererVerifyEmailSuccess(t *testing.T) {
	r := DefaultRenderer()
	req := withRendererTestExecutor(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/verify-email", nil))
	rec := httptest.NewRecorder()

	err := r.VerifyEmailSuccess(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "verify-success-title")
	assert.Contains(t, body, "card shadow-sm", "verify email success page should be wrapped in a card")
}

func TestDefaultRendererVerifyEmailError(t *testing.T) {
	r := DefaultRenderer()
	req := withRendererTestExecutor(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/verify-email", nil))
	rec := httptest.NewRecorder()

	err := r.VerifyEmailError(rec, req, "invalid_token")

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "verify-error-title")
	assert.Contains(t, body, "verify-error-invalid-token")
	assert.Contains(t, body, "card shadow-sm", "verify email error page should be wrapped in a card")
}

func TestDefaultRendererLoginPageWithLogo(t *testing.T) {
	r := DefaultRenderer()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/login", nil)

	logoExec := rendererTestExecutorWithLogo(`<span class="test-logo">My Brand</span>`)
	ctx := burrow.WithTemplateExecutor(req.Context(), logoExec)
	ctx = WithLogo(ctx, `<span class="test-logo">My Brand</span>`)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := r.LoginPage(rec, req, "/dashboard")

	require.NoError(t, err)
	body := rec.Body.String()
	assert.Contains(t, body, "test-logo")
	assert.Contains(t, body, "My Brand")
}

func TestDefaultRendererLoginPageWithoutLogo(t *testing.T) {
	r := DefaultRenderer()
	req := withRendererTestExecutor(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/login", nil))
	rec := httptest.NewRecorder()

	err := r.LoginPage(rec, req, "/dashboard")

	require.NoError(t, err)
	body := rec.Body.String()
	assert.NotContains(t, body, "text-center mb-4")
}

func TestDefaultRendererWithLayout(t *testing.T) {
	r := DefaultRenderer()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/login", nil)
	ctx := burrow.WithLayout(req.Context(), func(w http.ResponseWriter, r *http.Request, code int, content template.HTML, data map[string]any) error {
		return burrow.HTML(w, code, "<layout-wrapper>"+string(content)+"</layout-wrapper>")
	})
	ctx = burrow.WithTemplateExecutor(ctx, rendererTestExecutor())
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := r.LoginPage(rec, req, "/dashboard")

	require.NoError(t, err)
	assert.Contains(t, rec.Body.String(), "<layout-wrapper>")
	assert.Contains(t, rec.Body.String(), "login-button")
	assert.Contains(t, rec.Body.String(), "</layout-wrapper>")
}

func TestDefaultRendererIncludesCSRFToken(t *testing.T) {
	r := DefaultRenderer()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/login", nil)

	funcMap := template.FuncMap{
		"t":                    func(key string) string { return key },
		"csrfToken":            func() string { return "test-csrf-token-value" },
		"staticURL":            func(name string) string { return "/static/" + name },
		"authLogo":             func() template.HTML { return "" },
		"currentUser":          func() *User { return nil },
		"isAuthenticated":      func() bool { return false },
		"isAdminEditSelf":      func() bool { return false },
		"isAdminEditLastAdmin": func() bool { return false },
		"lang":                 func() string { return "en" },
		"itoa":                 func(id int64) string { return fmt.Sprintf("%d", id) },
		"credName":             credName,
		"emailValue":           func(user *User) string { return "" },
		"deref": func(s *string) string {
			if s != nil {
				return *s
			}
			return ""
		},
	}
	tmpl := template.Must(template.New("").Funcs(funcMap).Parse(`{{ define "bootstrap/theme_script" }}{{ end }}`))
	template.Must(tmpl.ParseFS(testRendererTemplateFS, "templates/*.html"))
	exec := func(r *http.Request, name string, data map[string]any) (template.HTML, error) {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
			return "", err
		}
		return template.HTML(buf.String()), nil //nolint:gosec // test
	}

	ctx := burrow.WithTemplateExecutor(req.Context(), exec)
	ctx = csrf.WithToken(ctx, "test-csrf-token-value")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := r.LoginPage(rec, req, "/dashboard")

	require.NoError(t, err)
	body := rec.Body.String()
	assert.Contains(t, body, `id="csrf-token"`)
	assert.Contains(t, body, "test-csrf-token-value")
}

// --- DefaultAuthLayout tests ---

func TestDefaultAuthLayoutWithoutExecutor(t *testing.T) {
	layout := DefaultAuthLayout()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	err := layout(rec, req, http.StatusOK, "<p>content</p>", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "<p>content</p>")
}

func TestDefaultAuthLayoutWithExecutor(t *testing.T) {
	layout := DefaultAuthLayout()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req = withRendererTestExecutor(req)
	rec := httptest.NewRecorder()

	err := layout(rec, req, http.StatusOK, "<p>test content</p>", map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestDefaultAuthLayoutWithTitle(t *testing.T) {
	layout := DefaultAuthLayout()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req = withRendererTestExecutor(req)
	rec := httptest.NewRecorder()

	err := layout(rec, req, http.StatusOK, "<p>content</p>", map[string]any{"Title": "My Title"})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- renderCentered/renderCard without executor ---

func TestDefaultRendererLoginPageWithoutExecutor(t *testing.T) {
	r := DefaultRenderer()
	// Request without template executor => falls back to burrow.RenderTemplate.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/login", nil)
	rec := httptest.NewRecorder()

	// This will try to use burrow.RenderTemplate which may return an error
	// if no template set is configured, but we're testing the code path.
	err := r.LoginPage(rec, req, "/dashboard")
	// Without a template executor, renderCentered falls through to burrow.RenderTemplate
	// which needs a template set. We just want to confirm it doesn't panic.
	_ = err
}

func TestDefaultRendererRegisterPageWithoutExecutor(t *testing.T) {
	r := DefaultRenderer()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/register", nil)
	rec := httptest.NewRecorder()

	err := r.RegisterPage(rec, req, false, false, "", "")
	_ = err // May error without templates, but should not panic.
}

// rendererTestExecutorWithLogo creates an executor where authLogo returns the given HTML.
func rendererTestExecutorWithLogo(logoHTML template.HTML) burrow.TemplateExecutor {
	funcMap := template.FuncMap{
		"t":                    func(key string) string { return key },
		"csrfToken":            func() string { return "test-csrf-token" },
		"staticURL":            func(name string) string { return "/static/" + name },
		"authLogo":             func() template.HTML { return logoHTML },
		"currentUser":          func() *User { return nil },
		"isAuthenticated":      func() bool { return false },
		"isAdminEditSelf":      func() bool { return false },
		"isAdminEditLastAdmin": func() bool { return false },
		"lang":                 func() string { return "en" },
		"itoa":                 func(id int64) string { return fmt.Sprintf("%d", id) },
		"credName":             credName,
		"emailValue":           func(user *User) string { return "" },
		"deref": func(s *string) string {
			if s != nil {
				return *s
			}
			return ""
		},
	}
	tmpl := template.Must(template.New("").Funcs(funcMap).Parse(`{{ define "bootstrap/theme_script" }}{{ end }}`))
	template.Must(tmpl.ParseFS(testRendererTemplateFS, "templates/*.html"))
	return func(r *http.Request, name string, data map[string]any) (template.HTML, error) {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
			return "", err
		}
		return template.HTML(buf.String()), nil //nolint:gosec // test
	}
}
