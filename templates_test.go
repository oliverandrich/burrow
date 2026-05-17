package burrow

import (
	"context"
	"html/template"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/document"
	"github.com/oliverandrich/den/storage/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseFuncMap(t *testing.T) {
	fm := baseFuncMap()

	assert.Contains(t, fm, "safeHTML")
	assert.Contains(t, fm, "safeURL")
	assert.Contains(t, fm, "safeAttr")
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

func TestBaseFuncMapDict(t *testing.T) {
	fm := baseFuncMap()
	fn := fm["dict"].(func(...any) map[string]any)

	t.Run("key value pairs", func(t *testing.T) {
		result := fn("a", 1, "b", "two")
		assert.Equal(t, map[string]any{"a": 1, "b": "two"}, result)
	})

	t.Run("odd number of args drops last", func(t *testing.T) {
		result := fn("a", 1, "orphan")
		assert.Equal(t, map[string]any{"a": 1}, result)
	})

	t.Run("non-string key skipped", func(t *testing.T) {
		result := fn(42, "val")
		assert.Empty(t, result)
	})

	t.Run("empty", func(t *testing.T) {
		result := fn()
		assert.Empty(t, result)
	})
}

func TestBaseFuncMapNoSilentDefaults(t *testing.T) {
	fm := baseFuncMap()

	// These funcs are provided by specific contrib apps (csrf) or by the
	// always-on i18n bundle (lang). The base FuncMap must not stub them,
	// otherwise a missing contrib silently degrades into an empty value.
	for _, name := range []string{"lang", "csrfToken", "csrfField", "csrfHxHeaders"} {
		assert.NotContains(t, fm, name, "base FuncMap must not stub %q", name)
	}
}

func TestCollectFuncMap_MediaURLFromStorage(t *testing.T) {
	fs, err := file.New(t.TempDir(), "/media/")
	require.NoError(t, err)
	t.Cleanup(func() { _ = fs.Close() })

	db, err := den.OpenURL(t.Context(), "sqlite://:memory:", den.WithStorage(fs))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	s := &Server{registry: NewRegistry(), appCfg: &AppConfig{DB: db}}
	fm, _ := s.collectFuncMap()

	fn, ok := fm["mediaURL"].(func(document.Attachment) string)
	require.True(t, ok, "mediaURL must be registered when DB has a Storage")
	assert.Equal(t, "/media/2026/04/x.jpg", fn(document.Attachment{StoragePath: "2026/04/x.jpg"}))
}

func TestCollectFuncMap_NoMediaURLWithoutStorage(t *testing.T) {
	db, err := den.OpenURL(t.Context(), "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	s := &Server{registry: NewRegistry(), appCfg: &AppConfig{DB: db}}
	fm, _ := s.collectFuncMap()

	_, ok := fm["mediaURL"]
	assert.False(t, ok, "mediaURL must not be registered when DB has no Storage")
}

func TestBuildTemplatesFailsWithoutCsrfApp(t *testing.T) {
	cases := []struct {
		name     string
		template string
	}{
		{"csrfToken", `{{ define "test/page" }}{{ csrfToken }}{{ end }}`},
		{"csrfField", `{{ define "test/page" }}{{ csrfField }}{{ end }}`},
		{"csrfHxHeaders", `{{ define "test/page" }}<body{{- csrfHxHeaders }}>{{ end }}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{registry: NewRegistry()}
			s.registry.Add(&templateApp{
				name:  "test",
				tplFS: fstest.MapFS{"page.html": &fstest.MapFile{Data: []byte(tc.template)}},
			})

			err := s.buildTemplates()
			require.Error(t, err, "template using %q must fail to parse when contrib/csrf is not registered", tc.name)
			assert.Contains(t, err.Error(), "function "+`"`+tc.name+`"`+" not defined")
		})
	}
}

func TestBuildTemplatesFailsWithoutLangFunc(t *testing.T) {
	s := &Server{registry: NewRegistry()}
	s.registry.Add(&templateApp{
		name:  "test",
		tplFS: fstest.MapFS{"page.html": &fstest.MapFile{Data: []byte(`{{ define "test/page" }}{{ lang }}{{ end }}`)}},
	})

	// Server.boot pre-registers i18n.Bundle which provides "lang"; this
	// test simulates the bare-Server case where boot hasn't run.
	err := s.buildTemplates()
	require.Error(t, err, "template using {{ lang }} must fail to parse without an i18n bundle pre-registered")
	assert.Contains(t, err.Error(), `function "lang" not defined`)
}

func TestCsrfTokenFromRequestFuncMap(t *testing.T) {
	s := &Server{registry: NewRegistry()}

	tplFS := fstest.MapFS{
		"page.html": &fstest.MapFile{
			Data: []byte(`{{ define "test/page" }}<body{{- csrfHxHeaders }}>{{ csrfToken }}{{ end }}`),
		},
	}

	app := &templateRequestFuncMapApp{
		name:  "csrf",
		tplFS: tplFS,
		rfm: func(_ context.Context) template.FuncMap {
			return template.FuncMap{
				"csrfToken":     func() string { return "real-token" },
				"csrfHxHeaders": func() template.HTMLAttr { return ` hx-headers='{"X-CSRF-Token":"real-token"}'` },
			}
		},
	}
	s.registry.Add(app)

	err := s.buildTemplates()
	require.NoError(t, err)

	html, err := s.executeTemplate(t.Context(), "test/page", nil)
	require.NoError(t, err)
	assert.Equal(t, template.HTML(`<body hx-headers='{"X-CSRF-Token":"real-token"}'>real-token`), html)
}

func TestParseTemplateFS_ReadError(t *testing.T) {
	badFS := fstest.MapFS{
		"dir/nested.html": &fstest.MapFile{
			Data: []byte(`{{ define "ok" }}ok{{ end }}`),
		},
	}
	tmpl := template.New("")
	// Valid FS should parse fine.
	require.NoError(t, parseTemplateFS(tmpl, badFS))
}

func TestParseTemplateFS_ParseError(t *testing.T) {
	badFS := fstest.MapFS{
		"broken.html": &fstest.MapFile{
			Data: []byte(`{{ define "broken" }}`),
		},
	}
	tmpl := template.New("")
	err := parseTemplateFS(tmpl, badFS)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse template broken.html")
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
		fm:    template.FuncMap{"add": func(a, b int) int { return a*10 + b }},
	}
	s.registry.Add(app)

	assert.NotPanics(t, func() { _ = s.buildTemplates() })
}

func TestBuildTemplatesDuplicateRequestFuncMapPanics(t *testing.T) {
	s := &Server{registry: NewRegistry()}

	app1 := &templateRequestFuncMapApp{
		name:  "csrf",
		tplFS: fstest.MapFS{},
		rfm: func(_ context.Context) template.FuncMap {
			return template.FuncMap{"token": func() string { return "a" }}
		},
	}
	app2 := &templateRequestFuncMapApp{
		name:  "other",
		tplFS: fstest.MapFS{},
		rfm: func(_ context.Context) template.FuncMap {
			return template.FuncMap{"token": func() string { return "b" }}
		},
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

	html, err := s.executeTemplate(t.Context(), "myapp/hello", map[string]any{"Name": "World"})
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
		rfm: func(_ context.Context) template.FuncMap {
			return template.FuncMap{
				"csrfToken": func() string { return "abc123" },
			}
		},
	}
	s.registry.Add(app)

	err := s.buildTemplates()
	require.NoError(t, err)

	html, err := s.executeTemplate(t.Context(), "myapp/page", nil)
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

	_, err = s.executeTemplate(t.Context(), "myapp/nonexistent", nil)
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
		{Label: "Home", URL: "/", Icon: "app/icon_home"},
	})

	links := buildNavLinks(ctx, "/")

	require.Len(t, links, 1)
	assert.Equal(t, "app/icon_home", links[0].Icon)
}

func TestIconFunc(t *testing.T) {
	s := &Server{registry: NewRegistry()}

	tplFS := fstest.MapFS{
		"icons.html": &fstest.MapFile{
			Data: []byte(`{{ define "test/icon_star" }}<svg>star</svg>{{ end }}`),
		},
		"page.html": &fstest.MapFile{
			Data: []byte(`{{ define "test/page" }}[{{ icon .Name }}]{{ end }}`),
		},
	}
	s.registry.Add(&templateApp{name: "test", tplFS: tplFS})

	require.NoError(t, s.buildTemplates())

	t.Run("known icon renders", func(t *testing.T) {
		html, err := s.executeTemplate(t.Context(), "test/page", map[string]any{"Name": "test/icon_star"})
		require.NoError(t, err)
		assert.Equal(t, template.HTML("[<svg>star</svg>]"), html)
	})

	t.Run("empty name renders nothing", func(t *testing.T) {
		html, err := s.executeTemplate(t.Context(), "test/page", map[string]any{"Name": ""})
		require.NoError(t, err)
		assert.Equal(t, template.HTML("[]"), html)
	})

	t.Run("unknown name renders nothing", func(t *testing.T) {
		html, err := s.executeTemplate(t.Context(), "test/page", map[string]any{"Name": "test/icon_missing"})
		require.NoError(t, err)
		assert.Equal(t, template.HTML("[]"), html)
	})
}

func TestBuildNavLinks_TranslatesLabelKey(t *testing.T) {
	ctx := context.Background()
	ctx = WithNavItems(ctx, []NavItem{
		{Label: "Fallback", LabelKey: "nav-home", URL: "/"},
	})

	// Without i18n configured, LabelKey returns itself — falls back to Label.
	links := buildNavLinks(ctx, "/")

	require.Len(t, links, 1)
	assert.Equal(t, "Fallback", links[0].Label, "should fall back to Label when translation equals key")
}

func TestBuildNavLinks_LabelKeyWithoutLabelFallsThrough(t *testing.T) {
	ctx := context.Background()
	ctx = WithNavItems(ctx, []NavItem{
		{LabelKey: "nav-untranslated", URL: "/"},
	})

	// No Label set, no i18n configured — LabelKey is returned as-is.
	links := buildNavLinks(ctx, "/")

	require.Len(t, links, 1)
	assert.Equal(t, "nav-untranslated", links[0].Label, "should use LabelKey as label when Label is empty and translation equals key")
}

func TestCoreRequestFuncMap_NavLinks(t *testing.T) {
	ctx := context.Background()
	ctx = WithNavItems(ctx, []NavItem{
		{Label: "Home", URL: "/", Position: 1},
	})
	ctx = WithRequestPath(ctx, "/")

	fm := coreRequestFuncMap(ctx)

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
		gotExec = TemplateExec(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.NotNil(t, gotExec, "template executor should be in context")
}

func TestExecuteTemplateUsingIconDoesNotPoisonClone(t *testing.T) {
	// Regression for "html/template: cannot Clone \"\" after it has executed":
	// {{ icon }} executes a template define out of the global tree. If it ran
	// against s.templates directly, the global tree would be marked executed
	// and the next Clone() (e.g. for layout wrap-around or any subsequent
	// request) would fail.
	s := &Server{registry: NewRegistry()}

	tplFS := fstest.MapFS{
		"page.html": &fstest.MapFile{
			Data: []byte(`{{ define "myapp/page" }}{{ icon "myapp/icon_star" }}{{ end }}`),
		},
		"icons.html": &fstest.MapFile{
			Data: []byte(`{{ define "myapp/icon_star" }}<svg>star</svg>{{ end }}`),
		},
	}

	// A request-scoped funcmap forces the executeTemplate Clone() path.
	app := &templateRequestFuncMapApp{
		name:  "myapp",
		tplFS: tplFS,
		rfm: func(_ context.Context) template.FuncMap {
			return template.FuncMap{"reqfn": func() string { return "" }}
		},
	}
	s.registry.Add(app)
	require.NoError(t, s.buildTemplates())

	html1, err := s.executeTemplate(t.Context(), "myapp/page", nil)
	require.NoError(t, err)
	assert.Equal(t, template.HTML("<svg>star</svg>"), html1)

	// Second execution would fail before the fix because s.templates was
	// marked executed by the icon call inside the first render.
	html2, err := s.executeTemplate(t.Context(), "myapp/page", nil)
	require.NoError(t, err, "Clone must still work after a render that used {{ icon }}")
	assert.Equal(t, template.HTML("<svg>star</svg>"), html2)
}

// Test helpers: apps implementing template interfaces.

type templateApp struct { //nolint:govet // fieldalignment: readability over optimization
	name  string
	tplFS fstest.MapFS
}

func (a *templateApp) Name() string      { return a.name }
func (a *templateApp) TemplateFS() fs.FS { return a.tplFS }

type templateFuncMapApp struct { //nolint:govet // fieldalignment: readability over optimization
	name  string
	tplFS fstest.MapFS
	fm    template.FuncMap
}

func (a *templateFuncMapApp) Name() string              { return a.name }
func (a *templateFuncMapApp) TemplateFS() fs.FS         { return a.tplFS }
func (a *templateFuncMapApp) FuncMap() template.FuncMap { return a.fm }

type templateRequestFuncMapApp struct { //nolint:govet // fieldalignment: readability over optimization
	name  string
	tplFS fstest.MapFS
	rfm   func(ctx context.Context) template.FuncMap
}

func (a *templateRequestFuncMapApp) Name() string      { return a.name }
func (a *templateRequestFuncMapApp) TemplateFS() fs.FS { return a.tplFS }
func (a *templateRequestFuncMapApp) RequestFuncMap(ctx context.Context) template.FuncMap {
	return a.rfm(ctx)
}

// Benchmarks

func BenchmarkExecuteTemplate_NoRequestFuncMap(b *testing.B) {
	s := &Server{registry: NewRegistry()}

	tplFS := fstest.MapFS{
		"page.html": &fstest.MapFile{
			Data: []byte(`{{ define "myapp/page" }}Hello, {{ .Name }}! You have {{ .Count }} items.{{ end }}`),
		},
	}
	s.registry.Add(&templateApp{name: "myapp", tplFS: tplFS})

	if err := s.buildTemplates(); err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()
	data := map[string]any{"Name": "World", "Count": 42}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = s.executeTemplate(ctx, "myapp/page", data)
	}
}

func BenchmarkExecuteTemplate_WithRequestFuncMap(b *testing.B) {
	s := &Server{registry: NewRegistry()}

	tplFS := fstest.MapFS{
		"page.html": &fstest.MapFile{
			Data: []byte(`{{ define "myapp/page" }}Token: {{ csrfToken }}. Lang: {{ currentLang }}. Hello, {{ .Name }}!{{ end }}`),
		},
	}

	app := &templateRequestFuncMapApp{
		name:  "myapp",
		tplFS: tplFS,
		rfm: func(_ context.Context) template.FuncMap {
			return template.FuncMap{
				"csrfToken":   func() string { return "abc123def456" },
				"currentLang": func() string { return "en" },
			}
		},
	}
	s.registry.Add(app)

	if err := s.buildTemplates(); err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()
	data := map[string]any{"Name": "World"}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = s.executeTemplate(ctx, "myapp/page", data)
	}
}

func BenchmarkExecuteTemplate_LargerTemplate(b *testing.B) {
	s := &Server{registry: NewRegistry()}

	// A more realistic template with multiple elements.
	tplFS := fstest.MapFS{
		"list.html": &fstest.MapFile{
			Data: []byte(`{{ define "myapp/list" }}<div class="container">` +
				`<h1>{{ .Title }}</h1>` +
				`<p>Showing page {{ .Page }} of {{ .TotalPages }}</p>` +
				`<ul>{{ range .Items }}<li>{{ . }}</li>{{ end }}</ul>` +
				`<nav>{{ if .HasPrev }}<a href="?page={{ sub .Page 1 }}">Prev</a>{{ end }}` +
				`{{ if .HasNext }}<a href="?page={{ add .Page 1 }}">Next</a>{{ end }}</nav>` +
				`</div>{{ end }}`),
		},
	}
	s.registry.Add(&templateApp{name: "myapp", tplFS: tplFS})

	if err := s.buildTemplates(); err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()
	data := map[string]any{
		"Title":      "All Items",
		"Page":       3,
		"TotalPages": 10,
		"Items":      []string{"Item 1", "Item 2", "Item 3", "Item 4", "Item 5"},
		"HasPrev":    true,
		"HasNext":    true,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = s.executeTemplate(ctx, "myapp/list", data)
	}
}
