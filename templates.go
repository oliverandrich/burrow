package burrow

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
)

// baseFuncMap returns the default template functions available in all templates.
func baseFuncMap() template.FuncMap {
	return template.FuncMap{
		"safeHTML": func(s string) template.HTML { return template.HTML(s) },         //nolint:gosec // intentional
		"safeURL":  func(s string) template.URL { return template.URL(s) },           //nolint:gosec // intentional
		"safeAttr": func(s string) template.HTMLAttr { return template.HTMLAttr(s) }, //nolint:gosec // intentional
		"itoa":     func(id int64) string { return strconv.FormatInt(id, 10) },
		"lang":     func() string { return "en" },
		"dict": func(pairs ...any) map[string]any {
			m := make(map[string]any, len(pairs)/2)
			for i := 0; i+1 < len(pairs); i += 2 {
				if key, ok := pairs[i].(string); ok {
					m[key] = pairs[i+1]
				}
			}
			return m
		},
	}
}

// coreRequestFuncMap provides request-scoped template functions for core
// framework values like navigation items.
func coreRequestFuncMap(r *http.Request) template.FuncMap {
	ctx := r.Context()
	var requestPath string
	if r.URL != nil {
		requestPath = r.URL.Path
	}
	return template.FuncMap{
		"navItems": func() []NavItem { return NavItems(ctx) },
		"navLinks": func() []NavLink { return buildNavLinks(ctx, requestPath) },
	}
}

// isActivePath reports whether requestPath matches itemURL for nav highlighting.
// The root URL "/" only matches exactly; other URLs match by prefix.
func isActivePath(requestPath, itemURL string) bool {
	if requestPath == "" {
		return false
	}
	if itemURL == "/" {
		return requestPath == "/"
	}
	return strings.HasPrefix(requestPath, itemURL)
}

// buildNavLinks filters NavItems by auth state and computes active status,
// returning template-ready NavLink values.
func buildNavLinks(ctx context.Context, requestPath string) []NavLink {
	items := NavItems(ctx)
	authenticated := isAuthenticated(ctx)
	admin := isAdmin(ctx)

	var links []NavLink
	for _, item := range items {
		if item.AuthOnly && !authenticated {
			continue
		}
		if item.AdminOnly && !admin {
			continue
		}
		links = append(links, NavLink{
			Label:    item.Label,
			URL:      item.URL,
			Icon:     item.Icon,
			IsActive: isActivePath(requestPath, item.URL),
		})
	}
	return links
}

// buildTemplates parses HTML templates from all HasTemplates apps into
// a single global template set. Static FuncMap entries from HasFuncMap
// apps are added at parse time. RequestFuncMap providers are collected
// for per-request cloning.
func (s *Server) buildTemplates() error {
	funcMap, templateFSes := s.collectFuncMap()

	if len(templateFSes) == 0 {
		return nil
	}

	t := template.New("").Funcs(funcMap)

	for _, fsys := range templateFSes {
		if err := parseTemplateFS(t, fsys); err != nil {
			return err
		}
	}

	s.templates = t
	return nil
}

// collectFuncMap builds the combined template.FuncMap from all registered apps
// and collects template file systems. It registers static FuncMap entries,
// collects RequestFuncMap providers (with stub functions for parse-time), and
// panics on duplicate function names across apps.
func (s *Server) collectFuncMap() (template.FuncMap, []fs.FS) {
	funcMap := baseFuncMap()
	baseKeys := make(map[string]bool, len(funcMap))
	for k := range funcMap {
		baseKeys[k] = true
	}

	checkDuplicate := func(k, source string) {
		if _, exists := funcMap[k]; exists && !baseKeys[k] {
			panic(fmt.Sprintf("burrow: duplicate template func %q registered by %s", k, source))
		}
	}

	// Register stubs for any pre-registered request func map providers
	// (e.g. the core i18n bundle, added before buildTemplates).
	stubReq := &http.Request{}
	for _, provider := range s.requestFuncMapProviders {
		for k := range provider(stubReq) {
			checkDuplicate(k, "core provider")
			funcMap[k] = func() string { return "" }
		}
	}

	var templateFSes []fs.FS

	for _, app := range s.registry.Apps() {
		if provider, ok := app.(HasFuncMap); ok {
			for k, v := range provider.FuncMap() {
				checkDuplicate(k, fmt.Sprintf("app %q", app.Name()))
				funcMap[k] = v
			}
		}
		if provider, ok := app.(HasRequestFuncMap); ok {
			s.requestFuncMapProviders = append(s.requestFuncMapProviders, provider.RequestFuncMap)
			// Register stub functions so templates can be parsed.
			// The real implementations are injected per request via Clone()+Funcs().
			for k := range provider.RequestFuncMap(stubReq) {
				checkDuplicate(k, fmt.Sprintf("app %q", app.Name()))
				funcMap[k] = func() string { return "" }
			}
		}
		if provider, ok := app.(HasTemplates); ok {
			templateFSes = append(templateFSes, provider.TemplateFS())
		}
	}

	return funcMap, templateFSes
}

// parseTemplateFS walks an fs.FS and parses all files as Go templates into t.
func parseTemplateFS(t *template.Template, fsys fs.FS) error {
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, readErr := fs.ReadFile(fsys, path)
		if readErr != nil {
			return fmt.Errorf("read template %s: %w", path, readErr)
		}
		_, parseErr := t.Parse(string(data))
		if parseErr != nil {
			return fmt.Errorf("parse template %s: %w", path, parseErr)
		}
		return nil
	})
}

// executeTemplate runs a named template with the given data. If any
// HasRequestFuncMap providers are registered, the template is cloned
// and request-scoped functions are added.
func (s *Server) executeTemplate(r *http.Request, name string, data map[string]any) (template.HTML, error) {
	t := s.templates

	if len(s.requestFuncMapProviders) > 0 {
		var err error
		t, err = t.Clone()
		if err != nil {
			return "", fmt.Errorf("clone templates: %w", err)
		}
		for _, provider := range s.requestFuncMapProviders {
			t.Funcs(provider(r))
		}
	}

	tmpl := t.Lookup(name)
	if tmpl == nil {
		return "", fmt.Errorf("template %q not found", name)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return template.HTML(buf.String()), nil //nolint:gosec // template output is trusted
}

// templateMiddleware returns middleware that injects the TemplateExecutor
// into the request context.
func (s *Server) templateMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := WithTemplateExecutor(r.Context(), s.executeTemplate)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
