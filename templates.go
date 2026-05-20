package burrow

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"

	"github.com/oliverandrich/burrow/i18n"
)

//go:embed templates/*.html
var coreTemplateFS embed.FS

// baseFuncMap returns the default template functions available in all templates.
//
// Only universal helpers live here. Funcs that depend on contrib registration
// (csrfToken, csrfField, csrfHxHeaders → contrib/csrf; staticURL →
// contrib/staticfiles; messages → contrib/messages) and the locale-scoped
// lang/t/tData/tPlural (provided by the always-on i18n bundle pre-registered
// by Server.boot) deliberately do NOT have stubs here — templates that use
// them fail to parse when the providing app is not registered, surfacing the
// missing dependency at boot instead of silently rendering empty values.
func baseFuncMap() template.FuncMap {
	return template.FuncMap{
		"safeHTML": func(s string) template.HTML { return template.HTML(s) },         //nolint:gosec // intentional
		"safeURL":  func(s string) template.URL { return template.URL(s) },           //nolint:gosec // intentional
		"safeAttr": func(s string) template.HTMLAttr { return template.HTMLAttr(s) }, //nolint:gosec // intentional
		"dict": func(pairs ...any) map[string]any {
			m := make(map[string]any, len(pairs)/2)
			for i := 0; i+1 < len(pairs); i += 2 {
				if key, ok := pairs[i].(string); ok {
					m[key] = pairs[i+1]
				}
			}
			return m
		},
		"add":         func(a, b int) int { return a + b },
		"sub":         func(a, b int) int { return a - b },
		"pageNumbers": PageNumbers,
		"pageURL":     PageURL,
	}
}

// coreRequestFuncMap provides context-scoped template functions for core
// framework values like navigation items.
func coreRequestFuncMap(ctx context.Context) template.FuncMap {
	requestPath := RequestPath(ctx)
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
			Label:    i18n.T(ctx, item.Label),
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

	// Parse core error templates first so apps can override them.
	coreFS, err := fs.Sub(coreTemplateFS, "templates")
	if err != nil {
		return fmt.Errorf("core templates: %w", err)
	}
	if err := parseTemplateFS(t, coreFS); err != nil {
		return err
	}

	for _, fsys := range templateFSes {
		if err := parseTemplateFS(t, fsys); err != nil {
			return err
		}
	}

	s.templates = t

	// iconTemplates is a sibling Clone used by the `icon` template function.
	// Executing icon defines against s.templates would mark it as executed,
	// which html/template forbids cloning afterwards — every subsequent
	// executeTemplate (which Clones s.templates per request) would then fail
	// with "cannot Clone ... after it has executed".
	iconTpl, err := t.Clone()
	if err != nil {
		return fmt.Errorf("clone icon templates: %w", err)
	}
	s.iconTemplates = iconTpl

	return nil
}

// collectFuncMap builds the combined template.FuncMap from all registered apps
// and collects template file systems. It registers static FuncMap entries,
// collects RequestFuncMap providers (with stub functions for parse-time), and
// panics on duplicate function names across apps.
func (s *Server) collectFuncMap() (template.FuncMap, []fs.FS) {
	funcMap := baseFuncMap()
	// icon renders an icon template by name. Used by layouts to render
	// NavItem.Icon / NavLink.Icon / DashboardItem.Icon, all of which hold
	// a template-define name (e.g. "auth/icon_people"). Apps keep their
	// icons in templates/icons.html as {{ define "<app>/icon_<name>" }} blocks.
	//
	// Executes against s.iconTemplates (a Clone made at buildTemplates time)
	// rather than s.templates so the icon execution doesn't mark s.templates
	// as executed — html/template forbids cloning a template that has been
	// executed, and we Clone s.templates per request. Icon defines see
	// parse-time stubs only; they cannot use request-scoped funcs (csrfToken,
	// t, currentUser, …). Acceptable: icons are static SVG snippets.
	funcMap["icon"] = func(name string) template.HTML {
		if name == "" || s.iconTemplates == nil {
			return ""
		}
		var buf bytes.Buffer
		if err := s.iconTemplates.ExecuteTemplate(&buf, name, nil); err != nil {
			return ""
		}
		return template.HTML(buf.String()) //nolint:gosec // template output, contextually escaped
	}
	// appName returns the configured human-readable application name (from
	// --app-name or the binary basename). Static across the process lifetime,
	// so it lives in the boot-time FuncMap rather than per-request.
	funcMap["appName"] = func() string {
		if s.appCfg == nil || s.appCfg.Config == nil {
			return ""
		}
		return s.appCfg.Config.Server.AppName
	}
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
	stubCtx := context.Background()
	for _, provider := range s.requestFuncMapProviders {
		for k := range provider(stubCtx) {
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
			for k := range provider.RequestFuncMap(stubCtx) {
				checkDuplicate(k, fmt.Sprintf("app %q", app.Name()))
				funcMap[k] = func() string { return "" }
			}
		}
		if provider, ok := app.(HasTemplates); ok {
			templateFSes = append(templateFSes, provider.TemplateFS())
		}
	}

	// Expose the configured den.Storage's URL composer as mediaURL when
	// a Storage is installed. Lets templates render attachments with
	// `{{ mediaURL .Hero }}` without any per-app FuncMap boilerplate;
	// works unchanged whether the backend is local (relative URL) or
	// remote (absolute URL).
	if s.appCfg != nil && s.appCfg.DB != nil {
		if st := s.appCfg.DB.Storage(); st != nil {
			checkDuplicate("mediaURL", "core (den.Storage)")
			funcMap["mediaURL"] = st.URL
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
// and context-scoped functions are added.
func (s *Server) executeTemplate(ctx context.Context, name string, data map[string]any) (template.HTML, error) {
	t := s.templates

	if len(s.requestFuncMapProviders) > 0 {
		var err error
		t, err = t.Clone()
		if err != nil {
			return "", fmt.Errorf("clone templates: %w", err)
		}
		for _, provider := range s.requestFuncMapProviders {
			t.Funcs(provider(ctx))
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
// and request path into the request context.
func (s *Server) templateMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ctx = WithRequestPath(ctx, r.URL.Path)
			ctx = WithTemplateExecutor(ctx, s.executeTemplate)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
