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
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/admin/modeladmin"
	matpl "github.com/oliverandrich/burrow/contrib/admin/modeladmin/templates"
	"github.com/oliverandrich/burrow/contrib/bsicons"

	"github.com/oliverandrich/burrow/contrib/session"
	"github.com/urfave/cli/v3"
)

//go:embed migrations
var migrationFS embed.FS

//go:embed translations
var translationFS embed.FS

//go:embed static
var staticFS embed.FS

//go:embed templates/*.html
var htmlTemplateFS embed.FS

// App implements the auth contrib app.
type App struct {
	repo          *Repository
	handlers      *Handlers
	usersAdmin    *modeladmin.ModelAdmin[User]
	invitesAdmin  *modeladmin.ModelAdmin[Invite]
	renderer      Renderer
	emailService  EmailService
	authLayout    burrow.LayoutFunc
	cancelCleanup context.CancelFunc
	config        *Config
	globalConfig  *burrow.Config
	withLocale    func(ctx context.Context, lang string) context.Context
	jobs          burrow.Queue
	logo          template.HTML
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
type Option func(*App)

// WithRenderer sets the page renderer for auth views.
func WithRenderer(r Renderer) Option {
	return func(a *App) { a.renderer = r }
}

// WithAuthLayout sets an optional layout for public (unauthenticated) auth pages.
// When set, pages like login, register, and recovery use this layout instead
// of the global app layout. Authenticated routes (credentials, recovery codes)
// continue to use the global layout.
func WithAuthLayout(fn burrow.LayoutFunc) Option {
	return func(a *App) { a.authLayout = fn }
}

// WithLogoComponent sets an optional logo HTML rendered above auth page content.
// When set, the logo appears on login, register, and recovery pages.
func WithLogoComponent(c template.HTML) Option {
	return func(a *App) { a.logo = c }
}

// WithEmailService sets the email service for the auth app.
func WithEmailService(e EmailService) Option {
	return func(a *App) { a.emailService = e }
}

// New creates a new auth app with the given options.
// By default, the built-in HTML renderer and auth layout are used.
// Use WithRenderer() and WithAuthLayout() to override.
func New(opts ...Option) *App {
	a := &App{
		renderer:   DefaultRenderer(),
		authLayout: DefaultAuthLayout(),
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

func (a *App) Name() string { return "auth" }

func (a *App) Dependencies() []string { return []string{"session"} }

func (a *App) Register(cfg *burrow.AppConfig) error {
	a.repo = NewRepository(cfg.DB)
	a.globalConfig = cfg.Config
	a.withLocale = cfg.WithLocale

	a.usersAdmin = &modeladmin.ModelAdmin[User]{
		Slug:              "users",
		DisplayName:       "User",
		DisplayPluralName: "Users",
		DB:                cfg.DB,
		Renderer:          matpl.DefaultRenderer[User](),
		CanCreate:         false,
		CanEdit:           false,
		CanDelete:         false,
		ListFields:        []string{"ID", "Username", "Name", "Email", "Role", "IsActive", "CreatedAt"},
		OrderBy:           "id DESC",
		Filters: []modeladmin.FilterDef{
			{Field: "role", Label: "Role", LabelKey: "admin-users-role", Type: "select", Choices: roleChoices()},
		},
		RowActions: []modeladmin.RowAction{
			{
				Slug:     "deactivate",
				Label:    "admin-users-action-deactivate",
				Icon:     bsicons.PersonSlash(),
				Method:   "POST",
				Class:    "btn-outline-secondary",
				Confirm:  "admin-users-deactivate-confirm",
				Handler:  deactivateUserHandler(a.repo),
				ShowWhen: isDeactivatable,
			},
			{
				Slug:     "activate",
				Label:    "admin-users-action-activate",
				Icon:     bsicons.PersonCheck(),
				Method:   "POST",
				Class:    "btn-outline-success",
				Handler:  activateUserHandler(a.repo),
				ShowWhen: isActivatable,
			},
			{
				Slug:    "delete",
				Label:   "modeladmin-delete",
				Icon:    bsicons.Trash(),
				Method:  "DELETE",
				Class:   "btn-outline-danger",
				Confirm: "admin-user-detail-delete-confirm",
				Handler: a.handleDeleteUser,
			},
		},
		EmptyMessageKey: "admin-users-none",
	}

	a.invitesAdmin = &modeladmin.ModelAdmin[Invite]{
		Slug:              "invites",
		DisplayName:       "Invite",
		DisplayPluralName: "Invites",
		DB:                cfg.DB,
		Renderer:          matpl.DefaultRenderer[Invite](),
		CanCreate:         true,
		CanEdit:           false,
		CanDelete:         false,
		ListFields:        []string{"ID", "Label", "Email", "ExpiresAt", "CreatedAt"},
		OrderBy:           "created_at DESC",
		RowActions: []modeladmin.RowAction{
			{
				Slug:     "revoke",
				Label:    "admin-invites-revoke",
				Method:   "DELETE",
				Icon:     bsicons.XCircle(),
				Class:    "btn-outline-danger",
				Confirm:  "admin-invites-revoke-confirm",
				Handler:  revokeInviteHandler(a.repo),
				ShowWhen: isRevokable,
			},
		},
		EmptyMessageKey: "admin-invites-none",
	}

	return nil
}

// roleChoices returns filter choices for the user role field.
func roleChoices() []modeladmin.Choice {
	return []modeladmin.Choice{
		{Value: RoleUser, Label: "User", LabelKey: "admin-user-detail-role-user"},
		{Value: RoleAdmin, Label: "Admin", LabelKey: "admin-user-detail-role-admin"},
	}
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

// TranslationFS returns the embedded translation files for auto-discovery by the i18n app.
func (a *App) TranslationFS() fs.FS { return translationFS }

func (a *App) Flags(configSource func(key string) cli.ValueSource) []cli.Flag {
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
			Name:    "webauthn-rp-id",
			Value:   "localhost",
			Usage:   "WebAuthn Relying Party ID (domain name)",
			Sources: burrow.FlagSources(configSource, "WEBAUTHN_RP_ID", "auth.webauthn_rp_id"),
		},
		&cli.StringFlag{
			Name:    "webauthn-rp-display-name",
			Value:   "Web App",
			Usage:   "WebAuthn Relying Party display name",
			Sources: burrow.FlagSources(configSource, "WEBAUTHN_RP_DISPLAY_NAME", "auth.webauthn_rp_display_name"),
		},
		&cli.StringFlag{
			Name:    "webauthn-rp-origin",
			Usage:   "WebAuthn Relying Party origin (defaults to base URL)",
			Sources: burrow.FlagSources(configSource, "WEBAUTHN_RP_ORIGIN", "auth.webauthn_rp_origin"),
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
		LogoutRedirect:      cmd.String("auth-logout-redirect"),
		UseEmail:            cmd.Bool("auth-use-email"),
		RequireVerification: cmd.Bool("auth-require-verification"),
		InviteOnly:          cmd.Bool("auth-invite-only"),
		BaseURL:             baseURL,
	}

	// Create WebAuthn service with a cancellable context for the cleanup goroutine.
	rpOrigin := cmd.String("webauthn-rp-origin")
	if rpOrigin == "" {
		rpOrigin = baseURL
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancelCleanup = cancel
	waSvc, err := NewWebAuthnService(
		ctx,
		cmd.String("webauthn-rp-display-name"),
		cmd.String("webauthn-rp-id"),
		rpOrigin,
	)
	if err != nil {
		return fmt.Errorf("create webauthn service: %w", err)
	}

	// Create handlers with the stored email service (if any).
	a.handlers = NewHandlers(a.repo, waSvc, a.emailService, a.renderer, a.config, a)

	// Start background cleanup of orphaned users from abandoned registrations.
	go a.cleanupOrphanedUsers(ctx)

	return nil
}

// cleanupOrphanedUsers periodically purges users with zero credentials that
// were created more than 5 minutes ago. These are leftover from abandoned
// WebAuthn registration flows.
func (a *App) cleanupOrphanedUsers(ctx context.Context) {
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
		}
	}
}

// Shutdown stops the background cleanup goroutine. Safe to call multiple
// times or if Configure was never called.
func (a *App) Shutdown(_ context.Context) error {
	if a.cancelCleanup != nil {
		a.cancelCleanup()
	}
	return nil
}

// TemplateFS returns the embedded HTML template files.
func (a *App) TemplateFS() fs.FS {
	sub, _ := fs.Sub(htmlTemplateFS, "templates")
	return sub
}

// FuncMap returns static template functions for auth templates.
func (a *App) FuncMap() template.FuncMap {
	return template.FuncMap{
		"credName": credName,
		"emailValue": func(user *User) string {
			if user.Email != nil {
				return *user.Email
			}
			return ""
		},
		"deref": func(s *string) string {
			if s != nil {
				return *s
			}
			return ""
		},
	}
}

// RequestFuncMap returns request-scoped template functions for auth state.
func (a *App) RequestFuncMap(r *http.Request) template.FuncMap {
	ctx := r.Context()
	return template.FuncMap{
		"currentUser":          func() *User { return UserFromContext(ctx) },
		"isAuthenticated":      func() bool { return IsAuthenticated(ctx) },
		"isAdminEditSelf":      func() bool { return IsAdminEditSelf(ctx) },
		"isAdminEditLastAdmin": func() bool { return IsAdminEditLastAdmin(ctx) },
		"authLogo":             func() template.HTML { return LogoFromContext(ctx) },
	}
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
		if err != nil || !user.IsActive {
			next.ServeHTTP(w, r)
			return
		}

		ctx := WithUser(r.Context(), user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Routes registers auth HTTP routes.
func (a *App) Routes(r chi.Router) {
	h := a.handlers

	r.Route("/auth", func(r chi.Router) {
		// Public routes — use auth layout and logo if set.
		r.Group(func(r chi.Router) {
			if a.authLayout != nil {
				r.Use(authLayoutMiddleware(a.authLayout))
			}
			if a.logo != "" {
				r.Use(authLogoMiddleware(a.logo))
			}
			r.Get("/register", burrow.Handle(h.RegisterPage))
			r.Post("/register/begin", burrow.Handle(h.RegisterBegin))
			r.Post("/register/finish", burrow.Handle(h.RegisterFinish))
			r.Get("/login", burrow.Handle(h.LoginPage))
			r.Post("/login/begin", burrow.Handle(h.LoginBegin))
			r.Post("/login/finish", burrow.Handle(h.LoginFinish))
			r.Post("/logout", burrow.Handle(h.Logout))
			r.Get("/recovery", burrow.Handle(h.RecoveryPage))
			r.Post("/recovery", burrow.Handle(h.RecoveryLogin))

			// Email verification routes.
			r.Get("/verify-pending", burrow.Handle(h.VerifyPendingPage))
			r.Get("/verify-email", burrow.Handle(h.VerifyEmail))
			r.Post("/resend-verification", burrow.Handle(h.ResendVerification))
		})

		// Authenticated credential management — keeps global layout.
		r.Route("/credentials", func(r chi.Router) {
			r.Use(RequireAuth())
			r.Get("/", burrow.Handle(h.CredentialsPage))
			r.Post("/begin", burrow.Handle(h.AddCredentialBegin))
			r.Post("/finish", burrow.Handle(h.AddCredentialFinish))
			r.Delete("/{id}", burrow.Handle(h.DeleteCredential))
		})

		// Authenticated recovery code management — keeps global layout.
		r.Route("/recovery-codes", func(r chi.Router) {
			r.Use(RequireAuth())
			r.Get("/", burrow.Handle(h.RecoveryCodesPage))
			r.Post("/ack", burrow.Handle(h.AcknowledgeRecoveryCodes))
			r.Post("/regenerate", burrow.Handle(h.RegenerateRecoveryCodes))
		})
	})
}

// authLayoutMiddleware overrides the layout in context for auth pages.
func authLayoutMiddleware(fn burrow.LayoutFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := burrow.WithLayout(r.Context(), fn)
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
func (a *App) AdminRoutes(r chi.Router) {
	r.Route("/users", func(r chi.Router) {
		r.Get("/", burrow.Handle(a.usersAdmin.HandleList))
		r.Get("/{id}", burrow.Handle(a.handleUserDetail))
		r.Post("/{id}", burrow.Handle(a.handleUpdateUser))
		r.Delete("/{id}", burrow.Handle(a.handleDeleteUser))
		r.Post("/{id}/deactivate", burrow.Handle(deactivateUserHandler(a.repo)))
		r.Post("/{id}/activate", burrow.Handle(activateUserHandler(a.repo)))
	})
	r.Route("/invites", func(r chi.Router) {
		r.Get("/", burrow.Handle(a.invitesAdmin.HandleList))
		r.Get("/new", burrow.Handle(a.invitesAdmin.HandleNew))
		r.Post("/", burrow.Handle(a.handleCreateInvite))
		r.Get("/{id}", burrow.Handle(a.invitesAdmin.HandleDetail))
		r.Delete("/{id}/revoke", burrow.Handle(revokeInviteHandler(a.repo)))
	})
}

// AdminNavItems returns navigation items for the admin panel.
func (a *App) AdminNavItems() []burrow.NavItem {
	return []burrow.NavItem{
		{
			Label:     "Users",
			LabelKey:  "admin-nav-users",
			URL:       "/admin/users",
			Icon:      bsicons.People(),
			Position:  10,
			AdminOnly: true,
		},
		{
			Label:     "Invites",
			LabelKey:  "admin-nav-invites",
			URL:       "/admin/invites",
			Icon:      bsicons.Envelope(),
			Position:  20,
			AdminOnly: true,
		},
	}
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

// credName returns a display name for a credential.
func credName(cred Credential) string {
	if cred.Name != "" {
		return cred.Name
	}
	return "Passkey"
}

// Repo returns the auth repository for external access.
func (a *App) Repo() *Repository { return a.repo }

// Handlers returns the auth handlers for external access.
func (a *App) Handlers() *Handlers { return a.handlers }

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
