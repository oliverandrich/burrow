// Package auth provides authentication as a webstack contrib app.
package auth

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"

	"codeberg.org/oliverandrich/go-webapp-template/contrib/session"
	"codeberg.org/oliverandrich/go-webapp-template/core"
	"github.com/labstack/echo/v5"
	"github.com/urfave/cli/v3"
)

//go:embed migrations
var migrationFS embed.FS

const storeKeyUser = "auth:user"

// GetUser retrieves the authenticated user from the Echo context.
func GetUser(c *echo.Context) *User {
	if user, ok := c.Get(storeKeyUser).(*User); ok {
		return user
	}
	return nil
}

// IsAuthenticated returns true if a user is logged in.
func IsAuthenticated(c *echo.Context) bool {
	return GetUser(c) != nil
}

// SetUser stores the user in the Echo context.
func SetUser(c *echo.Context, user *User) {
	c.Set(storeKeyUser, user)
}

// App implements the auth contrib app.
type App struct {
	repo         *Repository
	handlers     *Handlers
	renderer     Renderer
	config       *Config
	globalConfig *core.Config
}

// Config holds auth-specific configuration.
type Config struct { //nolint:govet // fieldalignment: readability over optimization
	LoginRedirect       string
	UseEmail            bool
	RequireVerification bool
	InviteOnly          bool
	BaseURL             string // Populated from global config at Configure time.
}

// New creates a new auth app with the given page renderer.
func New(renderer Renderer) *App {
	return &App{renderer: renderer}
}

func (a *App) Name() string { return "auth" }

func (a *App) Register(cfg *core.AppConfig) error {
	a.repo = NewRepository(cfg.DB)
	a.globalConfig = cfg.Config
	return nil
}

func (a *App) MigrationFS() fs.FS {
	sub, _ := fs.Sub(migrationFS, "migrations")
	return sub
}

func (a *App) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "auth-login-redirect",
			Value:   "/dashboard",
			Usage:   "Redirect target after successful login",
			Sources: cli.EnvVars("AUTH_LOGIN_REDIRECT"),
		},
		&cli.BoolFlag{
			Name:    "auth-use-email",
			Usage:   "Use email instead of username for authentication",
			Sources: cli.EnvVars("AUTH_USE_EMAIL"),
		},
		&cli.BoolFlag{
			Name:    "auth-require-verification",
			Usage:   "Require email verification before login",
			Sources: cli.EnvVars("AUTH_REQUIRE_VERIFICATION"),
		},
		&cli.BoolFlag{
			Name:    "auth-invite-only",
			Usage:   "Require an invite to register",
			Sources: cli.EnvVars("AUTH_INVITE_ONLY"),
		},
		&cli.StringFlag{
			Name:    "webauthn-rp-id",
			Value:   "localhost",
			Usage:   "WebAuthn Relying Party ID (domain name)",
			Sources: cli.EnvVars("WEBAUTHN_RP_ID"),
		},
		&cli.StringFlag{
			Name:    "webauthn-rp-display-name",
			Value:   "Web App",
			Usage:   "WebAuthn Relying Party display name",
			Sources: cli.EnvVars("WEBAUTHN_RP_DISPLAY_NAME"),
		},
		&cli.StringFlag{
			Name:    "webauthn-rp-origin",
			Usage:   "WebAuthn Relying Party origin (defaults to base URL)",
			Sources: cli.EnvVars("WEBAUTHN_RP_ORIGIN"),
		},
	}
}

func (a *App) Configure(cmd *cli.Command) error {
	baseURL := ""
	if a.globalConfig != nil {
		baseURL = a.globalConfig.ResolveBaseURL()
	}

	a.config = &Config{
		LoginRedirect:       cmd.String("auth-login-redirect"),
		UseEmail:            cmd.Bool("auth-use-email"),
		RequireVerification: cmd.Bool("auth-require-verification"),
		InviteOnly:          cmd.Bool("auth-invite-only"),
		BaseURL:             baseURL,
	}

	// Create WebAuthn service.
	rpOrigin := cmd.String("webauthn-rp-origin")
	if rpOrigin == "" {
		rpOrigin = baseURL
	}
	waSvc, err := NewWebAuthnService(
		cmd.String("webauthn-rp-display-name"),
		cmd.String("webauthn-rp-id"),
		rpOrigin,
	)
	if err != nil {
		return fmt.Errorf("create webauthn service: %w", err)
	}

	// Create handlers (email service is set via SetEmailService if needed).
	a.handlers = NewHandlers(a.repo, waSvc, nil, a.renderer, a.config)

	return nil
}

func (a *App) Middleware() []echo.MiddlewareFunc {
	return []echo.MiddlewareFunc{a.authMiddleware}
}

// authMiddleware loads the user from the session and sets it in the Echo context.
func (a *App) authMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		userID := session.GetInt64(c, "user_id")
		if userID == 0 {
			return next(c)
		}

		user, err := a.repo.GetUserByID(c.Request().Context(), userID)
		if err != nil {
			return next(c)
		}

		SetUser(c, user)
		return next(c)
	}
}

// RequireAuth returns middleware that redirects to login if not authenticated.
func RequireAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if !IsAuthenticated(c) {
				target := c.Request().URL.RequestURI()
				return c.Redirect(http.StatusSeeOther, "/auth/login?next="+url.QueryEscape(target))
			}
			return next(c)
		}
	}
}

// RequireAdmin returns middleware that returns 403 if the user is not an admin.
func RequireAdmin() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			user := GetUser(c)
			if user == nil || !user.IsAdmin() {
				return echo.NewHTTPError(http.StatusForbidden, "forbidden")
			}
			return next(c)
		}
	}
}

// Repo returns the auth repository for external access.
func (a *App) Repo() *Repository { return a.repo }

// Handlers returns the auth handlers for external access.
func (a *App) Handlers() *Handlers { return a.handlers }

// SetEmailService sets the email service for the auth app.
// Call this after Configure if email mode is enabled.
func (a *App) SetEmailService(email EmailService) {
	if a.handlers != nil {
		a.handlers.email = email
	}
}

// Routes registers auth HTTP routes.
func (a *App) Routes(e *echo.Echo) {
	h := a.handlers

	auth := e.Group("/auth")
	auth.GET("/register", h.RegisterPage)
	auth.POST("/register/begin", h.RegisterBegin)
	auth.POST("/register/finish", h.RegisterFinish)
	auth.GET("/login", h.LoginPage)
	auth.POST("/login/begin", h.LoginBegin)
	auth.POST("/login/finish", h.LoginFinish)
	auth.POST("/logout", h.Logout)
	auth.GET("/recovery", h.RecoveryPage)
	auth.POST("/recovery", h.RecoveryLogin)

	// Authenticated credential management.
	creds := auth.Group("/credentials", RequireAuth())
	creds.GET("", h.CredentialsPage)
	creds.POST("/begin", h.AddCredentialBegin)
	creds.POST("/finish", h.AddCredentialFinish)
	creds.DELETE("/:id", h.DeleteCredential)

	// Authenticated recovery code management.
	auth.POST("/recovery-codes/regenerate", h.RegenerateRecoveryCodes, RequireAuth())

	// Email verification routes.
	auth.GET("/verify-pending", h.VerifyPendingPage)
	auth.GET("/verify-email", h.VerifyEmail)
	auth.POST("/resend-verification", h.ResendVerification)

	// Admin invite management.
	admin := e.Group("/admin", RequireAuth(), RequireAdmin())
	admin.GET("/invites", h.InvitesPage)
	admin.POST("/invites", h.CreateInvite)
	admin.DELETE("/invites/:id", h.DeleteInvite)
}

// SafeRedirectPath validates a redirect path, falling back to defaultPath.
func SafeRedirectPath(next, defaultPath string) string {
	if next == "" {
		return defaultPath
	}
	parsed, err := url.Parse(next)
	if err != nil || parsed.Host != "" || parsed.Scheme != "" {
		return defaultPath
	}
	return next
}
