package auth

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"reflect"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/den/document"

	"github.com/oliverandrich/burrow/contrib/session"
	"github.com/urfave/cli/v3"
)

// resolveRPID picks the WebAuthn Relying-Party ID: the explicit flag value
// wins when non-empty, otherwise the host part of baseURL is derived
// automatically (port stripped). Returns "" only when both inputs are
// empty or baseURL fails to parse with no flag override — in which case
// gowebauthn.New rejects the config with a clear error.
func resolveRPID(flagValue, baseURL string) string {
	if flagValue != "" {
		return flagValue
	}
	if baseURL == "" {
		return ""
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// documentInterfaceType is the reflected document.Document marker
// interface, used by validateProfileType to detect Profile types that
// accidentally embed document.Base.
var documentInterfaceType = reflect.TypeFor[document.Document]()

// validateProfileType rejects Profile type parameters that satisfy the
// document.Document marker — i.e. types embedding document.Base. Profile
// is stored inline as JSON inside the user document; passing a Den
// document type would silently include Base's _id/_created_at/... fields
// in the inline payload and no separate table would be created. The
// error message points to the auth-profile doc for the correct pattern.
func validateProfileType[P any]() error {
	profileType := reflect.TypeFor[P]()
	if profileType.Implements(documentInterfaceType) {
		return fmt.Errorf("auth: Profile type %s must not embed document.Base — use a plain Go struct (see docs/contrib/auth-profile.md)", profileType)
	}
	return nil
}

//go:embed translations
var translationFS embed.FS

//go:embed static
var staticFS embed.FS

//go:embed templates/*.html
var htmlTemplateFS embed.FS

// App implements the auth contrib app. The Profile type parameter mirrors
// [User]'s — see docs/contrib/auth-profile.md.
type App[P any] struct {
	repo           *Repository[P]
	webauthn       WebAuthnService
	recovery       *RecoveryService
	renderer       Renderer
	emailService   EmailService
	authLayout     string
	cancelCleanup  context.CancelFunc
	cancelWebAuthn context.CancelFunc
	config         *Config
	globalConfig   *burrow.Config
	withLocale     func(ctx context.Context, lang string) context.Context
	emailTask      *burrow.TaskDefinition[emailJobPayload]
	logo           template.HTML
}

// Config holds auth-specific configuration.
type Config struct {
	LoginRedirect       string
	LogoutRedirect      string
	BaseURL             string
	UseEmail            bool
	RequireVerification bool
	InviteOnly          bool
}

// Option configures the auth app.
type Option[P any] func(*App[P])

// WithRenderer sets the page renderer for auth views.
func WithRenderer[P any](r Renderer) Option[P] {
	return func(a *App[P]) { a.renderer = r }
}

// WithAuthLayout sets an optional layout template name for public (unauthenticated)
// auth pages. When set, pages like login, register, and recovery use this layout
// instead of the global app layout. Authenticated routes (credentials, recovery codes)
// continue to use the global layout.
func WithAuthLayout[P any](name string) Option[P] {
	return func(a *App[P]) { a.authLayout = name }
}

// WithLogoComponent sets an optional logo HTML rendered above auth page content.
// When set, the logo appears on login, register, and recovery pages.
func WithLogoComponent[P any](c template.HTML) Option[P] {
	return func(a *App[P]) { a.logo = c }
}

// WithEmailService sets the email service for the auth app.
func WithEmailService[P any](e EmailService) Option[P] {
	return func(a *App[P]) { a.emailService = e }
}

// New creates a new auth app with the given options.
// By default, the built-in HTML renderer and auth layout are used.
// Use WithRenderer() and WithAuthLayout() to override.
func New[P any](opts ...Option[P]) *App[P] {
	a := &App[P]{
		renderer:   DefaultRenderer(),
		authLayout: DefaultAuthLayout(),
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

func (a *App[P]) Name() string { return "auth" }

func (a *App[P]) Dependencies() []string { return []string{"session", "csrf", "staticfiles"} }

func (a *App[P]) Configure(cfg *burrow.AppConfig, cmd *cli.Command) error {
	if err := validateProfileType[P](); err != nil {
		return err
	}

	a.repo = NewRepository[P](cfg.DB)
	a.globalConfig = cfg.Config
	a.withLocale = cfg.WithLocale

	// Read flag values and create config.
	baseURL := ""
	if a.globalConfig != nil {
		baseURL = a.globalConfig.ResolveBaseURL()
	}

	a.config = &Config{
		LoginRedirect:       cmd.String("auth-login-redirect"),
		LogoutRedirect:      cmd.String("auth-logout-redirect"),
		UseEmail:            cmd.Bool("auth-use-email"),
		RequireVerification: cmd.Bool("auth-require-verification"),
		InviteOnly:          cmd.Bool("auth-invite-only"),
		BaseURL:             baseURL,
	}

	// Create WebAuthn service.
	rpOrigin := cmd.String("auth-webauthn-rp-origin")
	if rpOrigin == "" {
		rpOrigin = baseURL
	}
	rpID := resolveRPID(cmd.String("auth-webauthn-rp-id"), baseURL)
	rpDisplayName := cmd.String("auth-webauthn-rp-display-name")
	if rpDisplayName == "" && a.globalConfig != nil {
		rpDisplayName = a.globalConfig.Server.AppName
	}
	waCtx, waCancel := context.WithCancel(context.Background())
	waSvc, err := NewWebAuthnService(
		waCtx,
		rpDisplayName,
		rpID,
		rpOrigin,
	)
	if err != nil {
		waCancel()
		return fmt.Errorf("create webauthn service: %w", err)
	}

	a.cancelWebAuthn = waCancel
	a.webauthn = waSvc
	a.recovery = NewRecoveryService()

	return nil
}

// Start launches the background cleanup goroutine after the full boot
// sequence completes. This ensures the goroutine only runs when the
// server has started successfully.
func (a *App[P]) Start(_ *burrow.Server) error {
	ctx, cancel := context.WithCancel(context.Background()) //nolint:gosec // cancel is stored on the App and invoked in Shutdown
	a.cancelCleanup = cancel
	go a.backgroundCleanup(ctx)
	return nil
}

// StaticFS returns the embedded static assets (webauthn.js) under the "auth" prefix.
func (a *App[P]) StaticFS() (string, fs.FS) {
	sub, _ := fs.Sub(staticFS, "static")
	return "auth", sub
}

// Documents returns the Den document types registered by this app.
func (a *App[P]) Documents() []document.Document {
	return []document.Document{&User[P]{}, &Credential{}, &RecoveryCode{}, &EmailVerificationToken{}, &Invite{}}
}

// TranslationFS returns the embedded translation files for auto-discovery by the i18n app.
func (a *App[P]) TranslationFS() fs.FS { return translationFS }

func (a *App[P]) Flags(configSource func(key string) cli.ValueSource) []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "auth-login-redirect",
			Value:   "/",
			Usage:   "Redirect target after successful login",
			Sources: burrow.FlagSources(configSource, "AUTH_LOGIN_REDIRECT", "auth.login_redirect"),
		},
		&cli.StringFlag{
			Name:    "auth-logout-redirect",
			Value:   "/auth/login",
			Usage:   "Redirect target after logout",
			Sources: burrow.FlagSources(configSource, "AUTH_LOGOUT_REDIRECT", "auth.logout_redirect"),
		},
		&cli.BoolFlag{
			Name:    "auth-use-email",
			Usage:   "Use email instead of username for authentication",
			Sources: burrow.FlagSources(configSource, "AUTH_USE_EMAIL", "auth.use_email"),
		},
		&cli.BoolFlag{
			Name:    "auth-require-verification",
			Usage:   "Require email verification before login",
			Sources: burrow.FlagSources(configSource, "AUTH_REQUIRE_VERIFICATION", "auth.require_verification"),
		},
		&cli.BoolFlag{
			Name:    "auth-invite-only",
			Usage:   "Require an invite to register",
			Sources: burrow.FlagSources(configSource, "AUTH_INVITE_ONLY", "auth.invite_only"),
		},
		&cli.StringFlag{
			Name:    "auth-webauthn-rp-id",
			Usage:   "WebAuthn Relying Party ID (defaults to the host of the base URL; override for registrable-suffix setups)",
			Sources: burrow.FlagSources(configSource, "WEBAUTHN_RP_ID", "auth.webauthn_rp_id"),
		},
		&cli.StringFlag{
			Name:    "auth-webauthn-rp-display-name",
			Usage:   "WebAuthn Relying Party display name (override; defaults to --app-name)",
			Sources: burrow.FlagSources(configSource, "WEBAUTHN_RP_DISPLAY_NAME", "auth.webauthn_rp_display_name"),
		},
		&cli.StringFlag{
			Name:    "auth-webauthn-rp-origin",
			Usage:   "WebAuthn Relying Party origin (defaults to base URL)",
			Sources: burrow.FlagSources(configSource, "WEBAUTHN_RP_ORIGIN", "auth.webauthn_rp_origin"),
		},
	}
}

// backgroundCleanup periodically purges orphaned users and expired email
// verification tokens. Orphaned users are leftover from abandoned WebAuthn
// registration flows (no credentials after 5 minutes).
func (a *App[P]) backgroundCleanup(ctx context.Context) {
	const (
		interval = 5 * time.Minute
		maxAge   = 5 * time.Minute
	)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			purged, err := a.repo.PurgeOrphanedUsers(ctx, maxAge)
			if err != nil {
				slog.Error("failed to purge orphaned users", "error", err)
			} else if purged > 0 {
				slog.Info("purged orphaned users", "count", purged)
			}

			if err := a.repo.DeleteExpiredEmailVerificationTokens(ctx); err != nil {
				slog.Error("failed to delete expired email verification tokens", "error", err)
			}
		}
	}
}

// Shutdown stops the background cleanup goroutine. Safe to call multiple
// times or if Configure was never called.
func (a *App[P]) Shutdown(_ context.Context) error {
	if a.cancelCleanup != nil {
		a.cancelCleanup()
	}
	if a.cancelWebAuthn != nil {
		a.cancelWebAuthn()
	}
	return nil
}

// TemplateFS returns the embedded HTML template files.
func (a *App[P]) TemplateFS() fs.FS {
	sub, _ := fs.Sub(htmlTemplateFS, "templates")
	return sub
}

// FuncMap returns static template functions for auth templates.
func (a *App[P]) FuncMap() template.FuncMap {
	return template.FuncMap{
		"credName": credName,
		"deref": func(s *string) string {
			if s != nil {
				return *s
			}
			return ""
		},
	}
}

// RequestFuncMap returns request-scoped template functions for auth state.
func (a *App[P]) RequestFuncMap(ctx context.Context) template.FuncMap {
	return template.FuncMap{
		"currentUser":     func() *User[P] { return CurrentUser[P](ctx) },
		"isAuthenticated": func() bool { return IsAuthenticated(ctx) },
		"authLogo":        func() template.HTML { return Logo(ctx) },
	}
}

func (a *App[P]) Middleware() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{a.authMiddleware}
}

// authMiddleware loads the user from the session and sets it in the request context.
// It also injects a burrow.AuthChecker so the core navLinks template function
// can filter AuthOnly/AdminOnly items without importing this package.
func (a *App[P]) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := session.GetString(r, "user_id")
		if userID == "" {
			next.ServeHTTP(w, r)
			return
		}

		user, err := a.repo.GetUserByID(r.Context(), userID)
		if err != nil || !user.IsActive {
			next.ServeHTTP(w, r)
			return
		}

		ctx := WithUser(r.Context(), user)
		ctx = burrow.WithAuthChecker(ctx, burrow.AuthChecker{
			IsAuthenticated: func() bool { return true },
			IsAdmin:         func() bool { return user.IsAdmin() },
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Routes registers auth HTTP routes.
func (a *App[P]) Routes(r chi.Router) {
	r.Route("/auth", func(r chi.Router) {
		// Public routes — use auth layout and logo if set.
		r.Group(func(r chi.Router) {
			if a.authLayout != "" {
				r.Use(authLayoutMiddleware(a.authLayout))
			}
			if a.logo != "" {
				r.Use(authLogoMiddleware(a.logo))
			}
			r.Get("/register", burrow.Handle(a.RegisterPage))
			r.Post("/register/begin", burrow.Handle(a.RegisterBegin))
			r.Post("/register/finish", burrow.Handle(a.RegisterFinish))
			r.Get("/login", burrow.Handle(a.LoginPage))
			r.Post("/login/begin", burrow.Handle(a.LoginBegin))
			r.Post("/login/finish", burrow.Handle(a.LoginFinish))
			r.Post("/logout", burrow.Handle(a.Logout))
			r.Get("/recovery", burrow.Handle(a.RecoveryPage))
			r.Post("/recovery", burrow.Handle(a.RecoveryLogin))

			// Email verification routes.
			r.Get("/verify-pending", burrow.Handle(a.VerifyPendingPage))
			r.Get("/verify-email", burrow.Handle(a.VerifyEmail))
			r.Post("/resend-verification", burrow.Handle(a.ResendVerification))
		})

		// Authenticated credential management — keeps global layout.
		r.Route("/credentials", func(r chi.Router) {
			r.Use(RequireAuth())
			r.Get("/", burrow.Handle(a.CredentialsPage))
			r.Post("/begin", burrow.Handle(a.AddCredentialBegin))
			r.Post("/finish", burrow.Handle(a.AddCredentialFinish))
			r.Delete("/{id}", burrow.Handle(a.DeleteCredential))
		})

		// Authenticated recovery code management — keeps global layout.
		r.Route("/recovery-codes", func(r chi.Router) {
			r.Use(RequireAuth())
			r.Get("/", burrow.Handle(a.RecoveryCodesPage))
			r.Post("/ack", burrow.Handle(a.AcknowledgeRecoveryCodes))
			r.Post("/regenerate", burrow.Handle(a.RegenerateRecoveryCodes))
		})
	})
}

// authLayoutMiddleware overrides the layout in context for auth pages.
func authLayoutMiddleware(name string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := burrow.WithLayout(r.Context(), name)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// authLogoMiddleware injects the logo HTML into the request context.
func authLogoMiddleware(logo template.HTML) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := WithLogo(r.Context(), logo)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AdminRoutes registers admin routes for user and invite management.
// The router is expected to already have auth middleware applied.
func (a *App[P]) AdminRoutes(r chi.Router) {
	// Users
	r.Get("/users", burrow.Handle(a.adminListUsers))
	r.Get("/users/{id}", burrow.Handle(a.adminEditUser))
	r.Post("/users/{id}", burrow.Handle(a.adminUpdateUser))
	r.Delete("/users/{id}", burrow.Handle(a.adminDeleteUser))
	r.Post("/users/{id}/deactivate", burrow.Handle(deactivateUserHandler(a.repo)))
	r.Post("/users/{id}/activate", burrow.Handle(activateUserHandler(a.repo)))

	// Invites
	r.Get("/invites", burrow.Handle(a.adminListInvites))
	r.Get("/invites/new", burrow.Handle(a.adminNewInviteForm))
	r.Post("/invites", burrow.Handle(a.handleCreateInvite))
	r.Delete("/invites/{id}/revoke", burrow.Handle(revokeInviteHandler(a.repo)))
}

// AdminNavItems returns navigation items for the admin panel.
func (a *App[P]) AdminNavItems() []burrow.NavItem {
	return []burrow.NavItem{
		{
			Label:     "Users",
			URL:       "/admin/users",
			Icon:      "auth/icon_people",
			Position:  10,
			AdminOnly: true,
		},
		{
			Label:     "Invites",
			URL:       "/admin/invites",
			Icon:      "auth/icon_envelope",
			Position:  20,
			AdminOnly: true,
		},
	}
}

// CLICommands returns auth-related CLI subcommands (promote, demote, create-invite).
func (a *App[P]) CLICommands() []*cli.Command {
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

// credName returns a display name for a credential.
func credName(cred Credential) string {
	if cred.Name != "" {
		return cred.Name
	}
	return "Passkey"
}

// Repo returns the auth repository for external access.
func (a *App[P]) Repo() *Repository[P] { return a.repo }

func (a *App[P]) setRole(ctx context.Context, cmd *cli.Command, role string) error {
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

func (a *App[P]) createInviteAction(ctx context.Context, cmd *cli.Command) error {
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
