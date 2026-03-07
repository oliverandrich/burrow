package templates

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/auth"
	"codeberg.org/oliverandrich/burrow/contrib/csrf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed *.html
var testTemplateFS embed.FS

// Compile-time interface assertions.
var (
	_ auth.Renderer      = (*defaultRenderer)(nil)
	_ auth.AdminRenderer = (*defaultAdminRenderer)(nil)
)

// testExecutor creates a TemplateExecutor for testing with stub functions.
func testExecutor() burrow.TemplateExecutor {
	funcMap := template.FuncMap{
		"t":                    func(key string) string { return key },
		"csrfToken":            func() string { return "test-csrf-token" },
		"staticURL":            func(name string) string { return "/static/" + name },
		"authLogo":             func() template.HTML { return "" },
		"currentUser":          func() *auth.User { return nil },
		"isAuthenticated":      func() bool { return false },
		"isAdminEditSelf":      func() bool { return false },
		"isAdminEditLastAdmin": func() bool { return false },
		"itoa":                 func(id int64) string { return template.HTMLEscapeString(fmt.Sprintf("%d", id)) },
		"credName":             credNameStub,
		"emailValue":           func(user *auth.User) string { return "" },
		"deref": func(s *string) string {
			if s != nil {
				return *s
			}
			return ""
		},
		"iconCheckCircleFill": func(class ...string) template.HTML { return "<svg>check-circle-fill</svg>" },
		"iconPeople":          func(class ...string) template.HTML { return "<svg>people</svg>" },
		"iconEnvelope":        func(class ...string) template.HTML { return "<svg>envelope</svg>" },
		"iconClipboard":       func(class ...string) template.HTML { return "<svg>clipboard</svg>" },
		"iconCheckLg":         func(class ...string) template.HTML { return "<svg>check-lg</svg>" },
	}

	// Parse stub for bootstrap/theme_script, then auth templates.
	tmpl := template.Must(template.New("").Funcs(funcMap).Parse(`{{ define "bootstrap/theme_script" }}{{ end }}`))
	template.Must(tmpl.ParseFS(testTemplateFS, "*.html"))

	return func(r *http.Request, name string, data map[string]any) (template.HTML, error) {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
			return "", err
		}
		return template.HTML(buf.String()), nil
	}
}

func credNameStub(cred auth.Credential) string {
	if cred.Name != "" {
		return cred.Name
	}
	return "Passkey"
}

// withTestExecutor sets up a request with a test template executor in context.
func withTestExecutor(req *http.Request) *http.Request {
	ctx := burrow.WithTemplateExecutor(req.Context(), testExecutor())
	return req.WithContext(ctx)
}

func TestDefaultRendererLoginPage(t *testing.T) {
	r := DefaultRenderer()
	req := withTestExecutor(httptest.NewRequest(http.MethodGet, "/auth/login", nil))
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
	req := withTestExecutor(httptest.NewRequest(http.MethodGet, "/auth/register", nil))
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
	req := withTestExecutor(httptest.NewRequest(http.MethodGet, "/auth/register", nil))
	rec := httptest.NewRecorder()

	err := r.RegisterPage(rec, req, true, false, "", "")

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "register-email-label")
	assert.NotContains(t, rec.Body.String(), "register-username-label")
}

func TestDefaultRendererCredentialsPage(t *testing.T) {
	r := DefaultRenderer()
	creds := []auth.Credential{
		{ID: 1, Name: "My Passkey"},
	}
	req := withTestExecutor(httptest.NewRequest(http.MethodGet, "/auth/credentials", nil))
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
	req := withTestExecutor(httptest.NewRequest(http.MethodGet, "/auth/recovery", nil))
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
	req := withTestExecutor(httptest.NewRequest(http.MethodGet, "/auth/recovery-codes", nil))
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
	req := withTestExecutor(httptest.NewRequest(http.MethodGet, "/auth/verify-pending", nil))
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
	req := withTestExecutor(httptest.NewRequest(http.MethodGet, "/auth/verify-email", nil))
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
	req := withTestExecutor(httptest.NewRequest(http.MethodGet, "/auth/verify-email", nil))
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
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)

	// Create an executor that reads logo from context.
	logoExec := testExecutorWithLogo(`<span class="test-logo">My Brand</span>`)
	ctx := burrow.WithTemplateExecutor(req.Context(), logoExec)
	ctx = auth.WithLogo(ctx, `<span class="test-logo">My Brand</span>`)
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
	req := withTestExecutor(httptest.NewRequest(http.MethodGet, "/auth/login", nil))
	rec := httptest.NewRecorder()

	err := r.LoginPage(rec, req, "/dashboard")

	require.NoError(t, err)
	body := rec.Body.String()
	// Without a logo, the logo wrapper div should not appear.
	assert.NotContains(t, body, "text-center mb-4")
}

func TestDefaultRendererWithLayout(t *testing.T) {
	r := DefaultRenderer()
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	// Set a layout in context.
	ctx := burrow.WithLayout(req.Context(), func(w http.ResponseWriter, r *http.Request, code int, content template.HTML, data map[string]any) error {
		return burrow.HTML(w, code, "<layout-wrapper>"+string(content)+"</layout-wrapper>")
	})
	ctx = burrow.WithTemplateExecutor(ctx, testExecutor())
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
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)

	// Create executor with real CSRF token stub.
	funcMap := template.FuncMap{
		"t":                    func(key string) string { return key },
		"csrfToken":            func() string { return "test-csrf-token-value" },
		"staticURL":            func(name string) string { return "/static/" + name },
		"authLogo":             func() template.HTML { return "" },
		"currentUser":          func() *auth.User { return nil },
		"isAuthenticated":      func() bool { return false },
		"isAdminEditSelf":      func() bool { return false },
		"isAdminEditLastAdmin": func() bool { return false },
		"itoa":                 func(id int64) string { return fmt.Sprintf("%d", id) },
		"credName":             credNameStub,
		"emailValue":           func(user *auth.User) string { return "" },
		"deref": func(s *string) string {
			if s != nil {
				return *s
			}
			return ""
		},
		"iconCheckCircleFill": func(class ...string) template.HTML { return "" },
		"iconPeople":          func(class ...string) template.HTML { return "" },
		"iconEnvelope":        func(class ...string) template.HTML { return "" },
		"iconClipboard":       func(class ...string) template.HTML { return "" },
		"iconCheckLg":         func(class ...string) template.HTML { return "" },
	}
	tmpl := template.Must(template.New("").Funcs(funcMap).Parse(`{{ define "bootstrap/theme_script" }}{{ end }}`))
	template.Must(tmpl.ParseFS(testTemplateFS, "*.html"))
	exec := func(r *http.Request, name string, data map[string]any) (template.HTML, error) {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
			return "", err
		}
		return template.HTML(buf.String()), nil
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

func TestDefaultAdminRendererIncludesCSRFToken(t *testing.T) {
	r := DefaultAdminRenderer()
	user := &auth.User{ID: 1, Username: "alice", Role: auth.RoleAdmin}
	req := httptest.NewRequest(http.MethodGet, "/admin/users/1", nil)

	// Create executor with real CSRF token.
	funcMap := template.FuncMap{
		"t":                    func(key string) string { return key },
		"csrfToken":            func() string { return "admin-csrf-token" },
		"staticURL":            func(name string) string { return "/static/" + name },
		"authLogo":             func() template.HTML { return "" },
		"currentUser":          func() *auth.User { return nil },
		"isAuthenticated":      func() bool { return false },
		"isAdminEditSelf":      func() bool { return false },
		"isAdminEditLastAdmin": func() bool { return false },
		"itoa":                 func(id int64) string { return fmt.Sprintf("%d", id) },
		"credName":             credNameStub,
		"emailValue": func(u *auth.User) string {
			if u.Email != nil {
				return *u.Email
			}
			return ""
		},
		"deref": func(s *string) string {
			if s != nil {
				return *s
			}
			return ""
		},
		"iconCheckCircleFill": func(class ...string) template.HTML { return "" },
		"iconPeople":          func(class ...string) template.HTML { return "" },
		"iconEnvelope":        func(class ...string) template.HTML { return "" },
		"iconClipboard":       func(class ...string) template.HTML { return "" },
		"iconCheckLg":         func(class ...string) template.HTML { return "" },
	}
	tmpl := template.Must(template.New("").Funcs(funcMap).Parse(`{{ define "bootstrap/theme_script" }}{{ end }}`))
	template.Must(tmpl.ParseFS(testTemplateFS, "*.html"))
	exec := func(r *http.Request, name string, data map[string]any) (template.HTML, error) {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
			return "", err
		}
		return template.HTML(buf.String()), nil
	}

	ctx := burrow.WithTemplateExecutor(req.Context(), exec)
	ctx = csrf.WithToken(ctx, "admin-csrf-token")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := r.AdminUserDetailPage(rec, req, user)

	require.NoError(t, err)
	body := rec.Body.String()
	assert.Contains(t, body, `name="gorilla.csrf.Token"`)
	assert.Contains(t, body, "admin-csrf-token")
}

func TestDefaultAdminRendererUsersPage(t *testing.T) {
	r := DefaultAdminRenderer()
	users := []auth.User{
		{ID: 1, Username: "alice", Role: auth.RoleUser},
		{ID: 2, Username: "bob", Role: auth.RoleAdmin},
	}
	req := withTestExecutor(httptest.NewRequest(http.MethodGet, "/admin/users", nil))
	rec := httptest.NewRecorder()

	err := r.AdminUsersPage(rec, req, users)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "alice")
	assert.Contains(t, rec.Body.String(), "bob")
}

func TestDefaultAdminRendererUserDetailPage(t *testing.T) {
	r := DefaultAdminRenderer()
	user := &auth.User{ID: 1, Username: "alice", Role: auth.RoleAdmin, Name: "Alice"}
	req := withTestExecutor(httptest.NewRequest(http.MethodGet, "/admin/users/1", nil))
	rec := httptest.NewRecorder()

	err := r.AdminUserDetailPage(rec, req, user)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "alice")
	assert.Contains(t, rec.Body.String(), "Alice")
}

func TestDefaultAdminRendererInvitesPage(t *testing.T) {
	r := DefaultAdminRenderer()
	invites := []auth.Invite{
		{ID: 1, Label: "John Doe", Email: "test@example.com", ExpiresAt: time.Now().Add(time.Hour)},
	}
	req := withTestExecutor(httptest.NewRequest(http.MethodGet, "/admin/invites", nil))
	rec := httptest.NewRecorder()

	err := r.AdminInvitesPage(rec, req, invites, "", false)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "John Doe")
	assert.NotContains(t, body, "test@example.com", "email column should be hidden when useEmail is false")
}

func TestDefaultAdminRendererInvitesPageWithCreatedURL(t *testing.T) {
	r := DefaultAdminRenderer()
	req := withTestExecutor(httptest.NewRequest(http.MethodGet, "/admin/invites", nil))
	rec := httptest.NewRecorder()

	err := r.AdminInvitesPage(rec, req, nil, "http://localhost/auth/register?invite=abc123", false)

	require.NoError(t, err)
	body := rec.Body.String()
	assert.Contains(t, body, "admin-invites-created")
	assert.Contains(t, body, "http://localhost/auth/register?invite=abc123")
	assert.Contains(t, body, "admin-invites-copy")
}

// testExecutorWithLogo creates an executor where authLogo returns the given HTML.
func testExecutorWithLogo(logoHTML template.HTML) burrow.TemplateExecutor {
	funcMap := template.FuncMap{
		"t":                    func(key string) string { return key },
		"csrfToken":            func() string { return "test-csrf-token" },
		"staticURL":            func(name string) string { return "/static/" + name },
		"authLogo":             func() template.HTML { return logoHTML },
		"currentUser":          func() *auth.User { return nil },
		"isAuthenticated":      func() bool { return false },
		"isAdminEditSelf":      func() bool { return false },
		"isAdminEditLastAdmin": func() bool { return false },
		"itoa":                 func(id int64) string { return fmt.Sprintf("%d", id) },
		"credName":             credNameStub,
		"emailValue":           func(user *auth.User) string { return "" },
		"deref": func(s *string) string {
			if s != nil {
				return *s
			}
			return ""
		},
		"iconCheckCircleFill": func(class ...string) template.HTML { return "" },
		"iconPeople":          func(class ...string) template.HTML { return "" },
		"iconEnvelope":        func(class ...string) template.HTML { return "" },
		"iconClipboard":       func(class ...string) template.HTML { return "" },
		"iconCheckLg":         func(class ...string) template.HTML { return "" },
	}
	tmpl := template.Must(template.New("").Funcs(funcMap).Parse(`{{ define "bootstrap/theme_script" }}{{ end }}`))
	template.Must(tmpl.ParseFS(testTemplateFS, "*.html"))
	return func(r *http.Request, name string, data map[string]any) (template.HTML, error) {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
			return "", err
		}
		return template.HTML(buf.String()), nil
	}
}
