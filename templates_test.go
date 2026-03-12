package burrow

import (
	"context"
	"html/template"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseFuncMap(t *testing.T) {
	fm := baseFuncMap()

	assert.Contains(t, fm, "safeHTML")
	assert.Contains(t, fm, "safeURL")
	assert.Contains(t, fm, "safeAttr")
	assert.Contains(t, fm, "itoa")
}

func TestBaseFuncMapItoa(t *testing.T) {
	fm := baseFuncMap()
	fn := fm["itoa"].(func(int64) string)
	assert.Equal(t, "42", fn(42))
	assert.Equal(t, "0", fn(0))
	assert.Equal(t, "-1", fn(-1))
}

func TestBaseFuncMapSafeHTML(t *testing.T) {
	fm := baseFuncMap()
	fn := fm["safeHTML"].(func(string) template.HTML)
	assert.Equal(t, template.HTML("<b>bold</b>"), fn("<b>bold</b>"))
}

func TestBaseFuncMapSafeURL(t *testing.T) {
	fm := baseFuncMap()
	fn := fm["safeURL"].(func(string) template.URL)
	assert.Equal(t, template.URL("https://example.com"), fn("https://example.com"))
}

func TestBaseFuncMapSafeAttr(t *testing.T) {
	fm := baseFuncMap()
	fn := fm["safeAttr"].(func(string) template.HTMLAttr)
	assert.Equal(t, template.HTMLAttr(`class="foo"`), fn(`class="foo"`))
}

func TestBuildTemplates(t *testing.T) {
	s := &Server{registry: NewRegistry()}

	tplFS := fstest.MapFS{
		"greeting.html": &fstest.MapFile{
			Data: []byte(`{{ define "myapp/greeting" }}Hello, {{ .Name }}!{{ end }}`),
		},
	}

	app := &templateApp{name: "myapp", tplFS: tplFS}
	s.registry.Add(app)

	err := s.buildTemplates()
	require.NoError(t, err)
	require.NotNil(t, s.templates)

	// Template should be findable by name.
	tpl := s.templates.Lookup("myapp/greeting")
	require.NotNil(t, tpl, "template myapp/greeting should exist")
}

func TestBuildTemplatesWithFuncMap(t *testing.T) {
	s := &Server{registry: NewRegistry()}

	tplFS := fstest.MapFS{
		"page.html": &fstest.MapFile{
			Data: []byte(`{{ define "myapp/page" }}{{ greet .Name }}{{ end }}`),
		},
	}

	app := &templateFuncMapApp{
		name:  "myapp",
		tplFS: tplFS,
		fm: template.FuncMap{
			"greet": func(name string) string { return "Hi, " + name + "!" },
		},
	}
	s.registry.Add(app)

	err := s.buildTemplates()
	require.NoError(t, err)
}

func TestBuildTemplatesDuplicateFuncMapPanics(t *testing.T) {
	s := &Server{registry: NewRegistry()}

	app1 := &templateFuncMapApp{
		name:  "app1",
		tplFS: fstest.MapFS{},
		fm:    template.FuncMap{"greet": func() string { return "hi" }},
	}
	app2 := &templateFuncMapApp{
		name:  "app2",
		tplFS: fstest.MapFS{},
		fm:    template.FuncMap{"greet": func() string { return "hello" }},
	}
	s.registry.Add(app1)
	s.registry.Add(app2)

	assert.PanicsWithValue(t,
		`burrow: duplicate template func "greet" registered by app "app2"`,
		func() { _ = s.buildTemplates() },
	)
}

func TestBuildTemplatesFuncMapOverridesBaseAllowed(t *testing.T) {
	s := &Server{registry: NewRegistry()}

	app := &templateFuncMapApp{
		name:  "override",
		tplFS: fstest.MapFS{},
		fm:    template.FuncMap{"lang": func() string { return "de" }},
	}
	s.registry.Add(app)

	assert.NotPanics(t, func() { _ = s.buildTemplates() })
}

func TestBuildTemplatesDuplicateRequestFuncMapPanics(t *testing.T) {
	s := &Server{registry: NewRegistry()}

	app1 := &templateRequestFuncMapApp{
		name:  "csrf",
		tplFS: fstest.MapFS{},
		rfm:   func(_ *http.Request) template.FuncMap { return template.FuncMap{"token": func() string { return "a" }} },
	}
	app2 := &templateRequestFuncMapApp{
		name:  "other",
		tplFS: fstest.MapFS{},
		rfm:   func(_ *http.Request) template.FuncMap { return template.FuncMap{"token": func() string { return "b" }} },
	}
	s.registry.Add(app1)
	s.registry.Add(app2)

	assert.PanicsWithValue(t,
		`burrow: duplicate template func "token" registered by app "other"`,
		func() { _ = s.buildTemplates() },
	)
}

func TestBuildTemplatesNoTemplateApps(t *testing.T) {
	s := &Server{registry: NewRegistry()}
	s.registry.Add(&minimalApp{})

	err := s.buildTemplates()
	require.NoError(t, err)
	assert.Nil(t, s.templates)
}

func TestExecuteTemplate(t *testing.T) {
	s := &Server{registry: NewRegistry()}

	tplFS := fstest.MapFS{
		"hello.html": &fstest.MapFile{
			Data: []byte(`{{ define "myapp/hello" }}Hello, {{ .Name }}!{{ end }}`),
		},
	}
	s.registry.Add(&templateApp{name: "myapp", tplFS: tplFS})

	err := s.buildTemplates()
	require.NoError(t, err)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	html, err := s.executeTemplate(r, "myapp/hello", map[string]any{"Name": "World"})
	require.NoError(t, err)
	assert.Equal(t, template.HTML("Hello, World!"), html)
}

func TestExecuteTemplateWithRequestFuncMap(t *testing.T) {
	s := &Server{registry: NewRegistry()}

	tplFS := fstest.MapFS{
		"page.html": &fstest.MapFile{
			Data: []byte(`{{ define "myapp/page" }}Token: {{ csrfToken }}{{ end }}`),
		},
	}

	app := &templateRequestFuncMapApp{
		name:  "myapp",
		tplFS: tplFS,
		rfm: func(r *http.Request) template.FuncMap {
			return template.FuncMap{
				"csrfToken": func() string { return "abc123" },
			}
		},
	}
	s.registry.Add(app)

	err := s.buildTemplates()
	require.NoError(t, err)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	html, err := s.executeTemplate(r, "myapp/page", nil)
	require.NoError(t, err)
	assert.Equal(t, template.HTML("Token: abc123"), html)
}

func TestExecuteTemplateNotFound(t *testing.T) {
	s := &Server{registry: NewRegistry()}

	tplFS := fstest.MapFS{
		"hello.html": &fstest.MapFile{
			Data: []byte(`{{ define "myapp/hello" }}Hello{{ end }}`),
		},
	}
	s.registry.Add(&templateApp{name: "myapp", tplFS: tplFS})

	err := s.buildTemplates()
	require.NoError(t, err)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	_, err = s.executeTemplate(r, "myapp/nonexistent", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestIsActivePath(t *testing.T) {
	tests := []struct {
		name        string
		requestPath string
		itemURL     string
		want        bool
	}{
		{"exact root match", "/", "/", true},
		{"root not active on subpath", "/notes", "/", false},
		{"prefix match", "/notes/1", "/notes", true},
		{"exact match", "/notes", "/notes", true},
		{"no match", "/settings", "/notes", false},
		{"empty request path", "", "/notes", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isActivePath(tt.requestPath, tt.itemURL))
		})
	}
}

func TestBuildNavLinks_PublicItems(t *testing.T) {
	ctx := context.Background()
	items := []NavItem{
		{Label: "Home", URL: "/", Position: 1},
		{Label: "About", URL: "/about", Position: 2},
	}
	ctx = WithNavItems(ctx, items)

	links := buildNavLinks(ctx, "/about")

	require.Len(t, links, 2)
	assert.Equal(t, "Home", links[0].Label)
	assert.False(t, links[0].IsActive)
	assert.Equal(t, "About", links[1].Label)
	assert.True(t, links[1].IsActive)
}

func TestBuildNavLinks_FiltersAuthOnly(t *testing.T) {
	ctx := context.Background()
	ctx = WithNavItems(ctx, []NavItem{
		{Label: "Home", URL: "/"},
		{Label: "Notes", URL: "/notes", AuthOnly: true},
	})

	links := buildNavLinks(ctx, "/")

	require.Len(t, links, 1)
	assert.Equal(t, "Home", links[0].Label)
}

func TestBuildNavLinks_ShowsAuthOnlyWhenAuthenticated(t *testing.T) {
	ctx := context.Background()
	ctx = WithNavItems(ctx, []NavItem{
		{Label: "Home", URL: "/"},
		{Label: "Notes", URL: "/notes", AuthOnly: true},
	})
	ctx = WithAuthChecker(ctx, AuthChecker{
		IsAuthenticated: func() bool { return true },
		IsAdmin:         func() bool { return false },
	})

	links := buildNavLinks(ctx, "/")

	require.Len(t, links, 2)
}

func TestBuildNavLinks_FiltersAdminOnly(t *testing.T) {
	ctx := context.Background()
	ctx = WithNavItems(ctx, []NavItem{
		{Label: "Admin", URL: "/admin", AdminOnly: true},
	})
	ctx = WithAuthChecker(ctx, AuthChecker{
		IsAuthenticated: func() bool { return true },
		IsAdmin:         func() bool { return false },
	})

	links := buildNavLinks(ctx, "/")

	assert.Empty(t, links)
}

func TestBuildNavLinks_ShowsAdminOnlyForAdmins(t *testing.T) {
	ctx := context.Background()
	ctx = WithNavItems(ctx, []NavItem{
		{Label: "Admin", URL: "/admin", AdminOnly: true},
	})
	ctx = WithAuthChecker(ctx, AuthChecker{
		IsAuthenticated: func() bool { return true },
		IsAdmin:         func() bool { return true },
	})

	links := buildNavLinks(ctx, "/")

	require.Len(t, links, 1)
	assert.Equal(t, "Admin", links[0].Label)
}

func TestBuildNavLinks_PreservesIcon(t *testing.T) {
	ctx := context.Background()
	ctx = WithNavItems(ctx, []NavItem{
		{Label: "Home", URL: "/", Icon: "<svg>icon</svg>"},
	})

	links := buildNavLinks(ctx, "/")

	require.Len(t, links, 1)
	assert.Equal(t, template.HTML("<svg>icon</svg>"), links[0].Icon)
}

func TestCoreRequestFuncMap_NavLinks(t *testing.T) {
	ctx := context.Background()
	ctx = WithNavItems(ctx, []NavItem{
		{Label: "Home", URL: "/", Position: 1},
	})
	r := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)

	fm := coreRequestFuncMap(r)

	navLinksFn, ok := fm["navLinks"].(func() []NavLink)
	require.True(t, ok)
	links := navLinksFn()
	require.Len(t, links, 1)
	assert.Equal(t, "Home", links[0].Label)
	assert.True(t, links[0].IsActive)
}

func TestTemplateMiddleware(t *testing.T) {
	s := &Server{registry: NewRegistry()}

	tplFS := fstest.MapFS{
		"hello.html": &fstest.MapFile{
			Data: []byte(`{{ define "myapp/hello" }}Hello{{ end }}`),
		},
	}
	s.registry.Add(&templateApp{name: "myapp", tplFS: tplFS})

	err := s.buildTemplates()
	require.NoError(t, err)

	var gotExec TemplateExecutor
	handler := s.templateMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotExec = TemplateExecutorFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.NotNil(t, gotExec, "template executor should be in context")
}

// Test helpers: apps implementing template interfaces.

type templateApp struct { //nolint:govet // fieldalignment: readability over optimization
	name  string
	tplFS fstest.MapFS
}

func (a *templateApp) Name() string                { return a.name }
func (a *templateApp) Register(_ *AppConfig) error { return nil }
func (a *templateApp) TemplateFS() fs.FS           { return a.tplFS }

type templateFuncMapApp struct { //nolint:govet // fieldalignment: readability over optimization
	name  string
	tplFS fstest.MapFS
	fm    template.FuncMap
}

func (a *templateFuncMapApp) Name() string                { return a.name }
func (a *templateFuncMapApp) Register(_ *AppConfig) error { return nil }
func (a *templateFuncMapApp) TemplateFS() fs.FS           { return a.tplFS }
func (a *templateFuncMapApp) FuncMap() template.FuncMap   { return a.fm }

type templateRequestFuncMapApp struct { //nolint:govet // fieldalignment: readability over optimization
	name  string
	tplFS fstest.MapFS
	rfm   func(r *http.Request) template.FuncMap
}

func (a *templateRequestFuncMapApp) Name() string                { return a.name }
func (a *templateRequestFuncMapApp) Register(_ *AppConfig) error { return nil }
func (a *templateRequestFuncMapApp) TemplateFS() fs.FS           { return a.tplFS }
func (a *templateRequestFuncMapApp) RequestFuncMap(r *http.Request) template.FuncMap {
	return a.rfm(r)
}
