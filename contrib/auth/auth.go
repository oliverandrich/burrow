// Package auth provides authentication as a burrow contrib app.
package auth

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"time"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/session"
	"github.com/go-chi/chi/v5"
	"github.com/urfave/cli/v3"
)

//go:embed migrations
var migrationFS embed.FS

// ctxKeyUser is the context key for the authenticated user.
type ctxKeyUser struct{}

// GetUser retrieves the authenticated user from the request context.
func GetUser(r *http.Request) *User {
	if user, ok := r.Context().Value(ctxKeyUser{}).(*User); ok {
		return user
	}
	return nil
}

// IsAuthenticated returns true if a user is logged in.
func IsAuthenticated(r *http.Request) bool {
	return GetUser(r) != nil
}

// WithUser returns a new context with the user set.
func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, ctxKeyUser{}, user)
}

// App implements the auth contrib app.
type App struct {
	repo          *Repository
	handlers      *Handlers
	adminHandlers *adminHandlers
	renderer      Renderer
	adminRenderer AdminRenderer
	config        *Config
	globalConfig  *burrow.Config
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

func (a *App) Dependencies() []string { return []string{"session"} }

func (a *App) Register(cfg *burrow.AppConfig) error {
	a.repo = NewRepository(cfg.DB)
	a.globalConfig = cfg.Config
	return nil
}

// StaticFS returns the embedded static assets (webauthn.js) under the "auth" prefix.
func (a *App) StaticFS() (string, fs.FS) {
	sub, _ := fs.Sub(staticFS, "static")
	return "auth", sub
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

func (a *App) Middleware() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{a.authMiddleware}
}

// authMiddleware loads the user from the session and sets it in the request context.
func (a *App) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := session.GetInt64(r, "user_id")
		if userID == 0 {
			next.ServeHTTP(w, r)
			return
		}

		user, err := a.repo.GetUserByID(r.Context(), userID)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		ctx := WithUser(r.Context(), user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAuth returns middleware that redirects to login if not authenticated.
func RequireAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !IsAuthenticated(r) {
				target := r.URL.RequestURI()
				http.Redirect(w, r, "/auth/login?next="+url.QueryEscape(target), http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin returns middleware that returns 403 if the user is not an admin.
func RequireAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUser(r)
			if user == nil || !user.IsAdmin() {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
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

// SetAdminRenderer sets the admin page renderer for user management.
// Call this after Register if admin rendering is needed.
func (a *App) SetAdminRenderer(r AdminRenderer) {
	a.adminRenderer = r
	if a.repo != nil && r != nil {
		var email EmailService
		if a.handlers != nil {
			email = a.handlers.email
		}
		a.adminHandlers = newAdminHandlers(a.repo, r, a.config, email)
	}
}

// AdminRoutes registers admin routes for user and invite management.
// The router is expected to already have auth middleware applied.
func (a *App) AdminRoutes(r chi.Router) {
	if a.adminHandlers == nil {
		return
	}
	h := a.adminHandlers

	r.Get("/users", burrow.Handle(h.UsersPage))
	r.Get("/users/{id}", burrow.Handle(h.UserDetail))
	r.Post("/users/{id}/role", burrow.Handle(h.UpdateUserRole))

	r.Get("/invites", burrow.Handle(h.InvitesPage))
	r.Post("/invites", burrow.Handle(h.CreateInvite))
	r.Delete("/invites/{id}", burrow.Handle(h.DeleteInvite))
}

// AdminNavItems returns navigation items for the admin panel.
func (a *App) AdminNavItems() []burrow.NavItem {
	return []burrow.NavItem{
		{
			Label:     "Users",
			URL:       "/admin/users",
			Icon:      "bi bi-people",
			Position:  10,
			AdminOnly: true,
		},
		{
			Label:     "Invites",
			URL:       "/admin/invites",
			Icon:      "bi bi-envelope",
			Position:  20,
			AdminOnly: true,
		},
	}
}

// Routes registers auth HTTP routes.
func (a *App) Routes(r chi.Router) {
	h := a.handlers

	r.Route("/auth", func(r chi.Router) {
		r.Get("/register", burrow.Handle(h.RegisterPage))
		r.Post("/register/begin", burrow.Handle(h.RegisterBegin))
		r.Post("/register/finish", burrow.Handle(h.RegisterFinish))
		r.Get("/login", burrow.Handle(h.LoginPage))
		r.Post("/login/begin", burrow.Handle(h.LoginBegin))
		r.Post("/login/finish", burrow.Handle(h.LoginFinish))
		r.Post("/logout", burrow.Handle(h.Logout))
		r.Get("/recovery", burrow.Handle(h.RecoveryPage))
		r.Post("/recovery", burrow.Handle(h.RecoveryLogin))

		// Authenticated credential management.
		r.Route("/credentials", func(r chi.Router) {
			r.Use(RequireAuth())
			r.Get("/", burrow.Handle(h.CredentialsPage))
			r.Post("/begin", burrow.Handle(h.AddCredentialBegin))
			r.Post("/finish", burrow.Handle(h.AddCredentialFinish))
			r.Delete("/{id}", burrow.Handle(h.DeleteCredential))
		})

		// Authenticated recovery code management.
		r.With(RequireAuth()).Post("/recovery-codes/regenerate", burrow.Handle(h.RegenerateRecoveryCodes))

		// Email verification routes.
		r.Get("/verify-pending", burrow.Handle(h.VerifyPendingPage))
		r.Get("/verify-email", burrow.Handle(h.VerifyEmail))
		r.Post("/resend-verification", burrow.Handle(h.ResendVerification))
	})
}

// CLICommands returns auth-related CLI subcommands (promote, demote, create-invite).
func (a *App) CLICommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:      "promote",
			Usage:     "Promote a user to admin",
			ArgsUsage: "<username>",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				return a.setRole(ctx, cmd, RoleAdmin)
			},
		},
		{
			Name:      "demote",
			Usage:     "Demote an admin to regular user",
			ArgsUsage: "<username>",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				return a.setRole(ctx, cmd, RoleUser)
			},
		},
		{
			Name:      "create-invite",
			Usage:     "Create an invite and print the registration URL",
			ArgsUsage: "<email>",
			Action:    a.createInviteAction,
		},
	}
}

func (a *App) setRole(ctx context.Context, cmd *cli.Command, role string) error {
	username := cmd.Args().First()
	if username == "" {
		return fmt.Errorf("username is required")
	}

	if a.repo == nil {
		return fmt.Errorf("auth app not initialized")
	}

	user, err := a.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return fmt.Errorf("user %q not found: %w", username, err)
	}

	if err := a.repo.SetUserRole(ctx, user.ID, role); err != nil {
		return fmt.Errorf("set role: %w", err)
	}

	fmt.Printf("User %q role set to %q\n", username, role)
	return nil
}

func (a *App) createInviteAction(ctx context.Context, cmd *cli.Command) error {
	inviteEmail := cmd.Args().First()
	if inviteEmail == "" {
		return fmt.Errorf("email is required")
	}

	if a.repo == nil {
		return fmt.Errorf("auth app not initialized")
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return fmt.Errorf("generate random bytes: %w", err)
	}
	plainToken := hex.EncodeToString(tokenBytes)
	tokenHash := HashToken(plainToken)

	baseURL := ""
	if a.globalConfig != nil {
		baseURL = a.globalConfig.ResolveBaseURL()
	}

	invite := &Invite{
		Email:     inviteEmail,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(InviteExpiry),
	}

	if err := a.repo.CreateInvite(ctx, invite); err != nil {
		return fmt.Errorf("create invite: %w", err)
	}

	registrationURL := fmt.Sprintf("%s/auth/register?invite=%s", baseURL, plainToken)

	fmt.Printf("Invite created for %s\n", inviteEmail)
	fmt.Printf("Registration URL: %s\n", registrationURL)
	fmt.Printf("Expires: %s\n", invite.ExpiresAt.Format(time.RFC3339))

	return nil
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
