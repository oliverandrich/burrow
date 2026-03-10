package secure

import (
	"net/http"
	"strings"

	"github.com/oliverandrich/burrow"
	"github.com/unrolled/secure"
	"github.com/urfave/cli/v3"
)

// Option configures the secure app.
type Option func(*App)

// WithContentSecurityPolicy sets the Content-Security-Policy header.
func WithContentSecurityPolicy(csp string) Option {
	return func(a *App) { a.csp = &csp }
}

// WithPermissionsPolicy sets the Permissions-Policy header.
func WithPermissionsPolicy(pp string) Option {
	return func(a *App) { a.permissionsPolicy = &pp }
}

// WithCrossOriginOpenerPolicy sets the Cross-Origin-Opener-Policy header.
func WithCrossOriginOpenerPolicy(coop string) Option {
	return func(a *App) { a.coop = &coop }
}

// WithAllowedHosts sets the list of allowed hostnames for Host header validation.
func WithAllowedHosts(hosts ...string) Option {
	return func(a *App) { a.allowedHosts = &hosts }
}

// WithSSLRedirect enables or disables HTTP-to-HTTPS redirect.
func WithSSLRedirect(redirect bool) Option {
	return func(a *App) { a.sslRedirect = &redirect }
}

// WithSSLProxyHeaders sets proxy headers used to detect HTTPS behind a reverse proxy.
func WithSSLProxyHeaders(headers map[string]string) Option {
	return func(a *App) { a.sslProxyHeaders = headers }
}

// WithDevelopment forces development mode on or off, overriding auto-detection.
func WithDevelopment(dev bool) Option {
	return func(a *App) { a.development = &dev }
}

// New creates a new secure headers app with the given options.
func New(opts ...Option) *App {
	a := &App{}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// App implements security response headers as a burrow contrib app.
type App struct {
	config            *burrow.Config
	handler           func(http.Handler) http.Handler
	csp               *string
	permissionsPolicy *string
	coop              *string
	allowedHosts      *[]string
	sslRedirect       *bool
	sslProxyHeaders   map[string]string
	development       *bool
}

func (a *App) Name() string { return "secure" }

func (a *App) Register(cfg *burrow.AppConfig) error {
	a.config = cfg.Config
	return nil
}

func (a *App) Flags(configSource func(key string) cli.ValueSource) []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "secure-csp",
			Usage:   "Content-Security-Policy header value",
			Sources: burrow.FlagSources(configSource, "SECURE_CSP", "secure.csp"),
		},
		&cli.StringFlag{
			Name:    "secure-permissions-policy",
			Usage:   "Permissions-Policy header value",
			Sources: burrow.FlagSources(configSource, "SECURE_PERMISSIONS_POLICY", "secure.permissions_policy"),
		},
		&cli.StringFlag{
			Name:    "secure-coop",
			Usage:   "Cross-Origin-Opener-Policy header value",
			Sources: burrow.FlagSources(configSource, "SECURE_COOP", "secure.coop"),
		},
		&cli.StringFlag{
			Name:    "secure-allowed-hosts",
			Usage:   "Comma-separated list of allowed hostnames",
			Sources: burrow.FlagSources(configSource, "SECURE_ALLOWED_HOSTS", "secure.allowed_hosts"),
		},
		&cli.BoolFlag{
			Name:    "secure-ssl-redirect",
			Usage:   "Redirect HTTP requests to HTTPS",
			Sources: burrow.FlagSources(configSource, "SECURE_SSL_REDIRECT", "secure.ssl_redirect"),
		},
		&cli.BoolFlag{
			Name:    "secure-development",
			Usage:   "Force development mode (disables HSTS, SSL redirect, host checks)",
			Sources: burrow.FlagSources(configSource, "SECURE_DEVELOPMENT", "secure.development"),
		},
	}
}

func (a *App) Configure(cmd *cli.Command) error {
	isHTTPS := a.config != nil && strings.HasPrefix(a.config.Server.BaseURL, "https://")

	if a.csp == nil {
		if v := cmd.String("secure-csp"); v != "" {
			a.csp = &v
		}
	}
	if a.permissionsPolicy == nil {
		if v := cmd.String("secure-permissions-policy"); v != "" {
			a.permissionsPolicy = &v
		}
	}
	if a.coop == nil {
		if v := cmd.String("secure-coop"); v != "" {
			a.coop = &v
		}
	}
	if a.allowedHosts == nil {
		if v := cmd.String("secure-allowed-hosts"); v != "" {
			hosts := strings.Split(v, ",")
			for i := range hosts {
				hosts[i] = strings.TrimSpace(hosts[i])
			}
			a.allowedHosts = &hosts
		}
	}
	if a.sslRedirect == nil {
		if cmd.IsSet("secure-ssl-redirect") {
			v := cmd.Bool("secure-ssl-redirect")
			a.sslRedirect = &v
		}
	}
	if a.development == nil {
		if cmd.IsSet("secure-development") {
			v := cmd.Bool("secure-development")
			a.development = &v
		}
	}

	a.configure(isHTTPS)
	return nil
}

// configure builds the unrolled/secure middleware. Extracted for testability.
func (a *App) configure(isHTTPS bool) {
	opts := secure.Options{
		ContentTypeNosniff: true,
		FrameDeny:          true,
		ReferrerPolicy:     "strict-origin-when-cross-origin",
	}

	if isHTTPS {
		opts.STSSeconds = 63072000 // 2 years
		opts.STSIncludeSubdomains = true
		opts.STSPreload = true
	}

	if a.csp != nil {
		opts.ContentSecurityPolicy = *a.csp
	}
	if a.permissionsPolicy != nil {
		opts.PermissionsPolicy = *a.permissionsPolicy
	}
	if a.coop != nil {
		opts.CrossOriginOpenerPolicy = *a.coop
	}
	if a.allowedHosts != nil {
		opts.AllowedHosts = *a.allowedHosts
	}
	if a.sslRedirect != nil {
		opts.SSLRedirect = *a.sslRedirect
	}
	if a.sslProxyHeaders != nil {
		opts.SSLProxyHeaders = a.sslProxyHeaders
	}

	// Development mode: explicit setting wins, otherwise auto-detect from BaseURL.
	if a.development != nil {
		opts.IsDevelopment = *a.development
	} else if !isHTTPS {
		opts.IsDevelopment = true
	}

	s := secure.New(opts)
	a.handler = s.Handler
}

func (a *App) Middleware() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{a.handler}
}
