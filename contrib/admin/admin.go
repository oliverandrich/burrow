// Package admin provides user management as a webstack contrib app.
package admin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"codeberg.org/oliverandrich/go-webapp-template/contrib/auth"
	"codeberg.org/oliverandrich/go-webapp-template/core"
	"github.com/labstack/echo/v5"
	"github.com/urfave/cli/v3"
)

// Store defines data operations needed by the admin app.
type Store interface {
	ListUsers(ctx context.Context) ([]auth.User, error)
	GetUserByID(ctx context.Context, id int64) (*auth.User, error)
	SetUserRole(ctx context.Context, userID int64, role string) error
	GetUserByUsername(ctx context.Context, username string) (*auth.User, error)
	CreateInvite(ctx context.Context, invite *auth.Invite) error
}

// App implements the admin contrib app.
type App struct {
	authRepo *auth.Repository
	handlers *Handlers
	baseURL  string
}

// New creates a new admin app.
func New() *App {
	return &App{}
}

func (a *App) Name() string { return "admin" }

func (a *App) Register(cfg *core.AppConfig) error {
	authApp, ok := cfg.Registry.Get("auth")
	if !ok {
		return fmt.Errorf("admin app requires auth app to be registered")
	}

	if repoProvider, ok := authApp.(interface{ Repo() *auth.Repository }); ok {
		a.authRepo = repoProvider.Repo()
	}

	if cfg.Config != nil {
		a.baseURL = cfg.Config.ResolveBaseURL()
	}

	return nil
}

func (a *App) NavItems() []core.NavItem {
	return []core.NavItem{
		{
			Label:     "Users",
			URL:       "/admin/users",
			Icon:      "bi bi-people",
			Position:  90,
			AdminOnly: true,
		},
	}
}

func (a *App) Routes(e *echo.Echo) {
	if a.handlers == nil {
		return
	}
	h := a.handlers

	admin := e.Group("/admin", auth.RequireAuth(), auth.RequireAdmin())
	admin.GET("/users", h.UsersPage)
	admin.GET("/users/:id", h.UserDetail)
	admin.POST("/users/:id/role", h.UpdateUserRole)
}

// SetHandlers sets the handlers for the admin app.
// Call this after Register if a renderer is available.
func (a *App) SetHandlers(renderer Renderer) {
	if a.authRepo != nil {
		a.handlers = NewHandlers(a.authRepo, renderer)
	}
}

// Handlers returns the admin handlers for external access.
func (a *App) Handlers() *Handlers { return a.handlers }

func (a *App) CLICommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:      "promote",
			Usage:     "Promote a user to admin",
			ArgsUsage: "<username>",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				return a.setRole(ctx, cmd, auth.RoleAdmin)
			},
		},
		{
			Name:      "demote",
			Usage:     "Demote an admin to regular user",
			ArgsUsage: "<username>",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				return a.setRole(ctx, cmd, auth.RoleUser)
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

	if a.authRepo == nil {
		return fmt.Errorf("admin app not initialized")
	}

	user, err := a.authRepo.GetUserByUsername(ctx, username)
	if err != nil {
		return fmt.Errorf("user %q not found: %w", username, err)
	}

	if err := a.authRepo.SetUserRole(ctx, user.ID, role); err != nil {
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

	if a.authRepo == nil {
		return fmt.Errorf("admin app not initialized")
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return fmt.Errorf("generate random bytes: %w", err)
	}
	plainToken := hex.EncodeToString(tokenBytes)
	tokenHash := auth.HashToken(plainToken)

	invite := &auth.Invite{
		Email:     inviteEmail,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(auth.InviteExpiry),
	}

	if err := a.authRepo.CreateInvite(ctx, invite); err != nil {
		return fmt.Errorf("create invite: %w", err)
	}

	registrationURL := fmt.Sprintf("%s/auth/register?invite=%s", a.baseURL, plainToken)

	fmt.Printf("Invite created for %s\n", inviteEmail)
	fmt.Printf("Registration URL: %s\n", registrationURL)
	fmt.Printf("Expires: %s\n", invite.ExpiresAt.Format(time.RFC3339))

	return nil
}
