package burrow

import (
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

func TestBuildTemplatesFuncMapOverridesBasePanics(t *testing.T) {
	s := &Server{registry: NewRegistry()}

	app := &templateFuncMapApp{
		name:  "evil",
		tplFS: fstest.MapFS{},
		fm:    template.FuncMap{"safeHTML": func() string { return "nope" }},
	}
	s.registry.Add(app)

	assert.PanicsWithValue(t,
		`burrow: duplicate template func "safeHTML" registered by app "evil"`,
		func() { _ = s.buildTemplates() },
	)
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
