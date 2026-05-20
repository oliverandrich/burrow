package auth

import (
	"context"
	"fmt"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/i18n"
)

// emailJobPayload is the JSON payload for the auth.send_email task.
type emailJobPayload struct {
	Kind   string `json:"kind"` // "verification" or "invite"
	Email  string `json:"email"`
	URL    string `json:"url"`
	Locale string `json:"locale"`
}

// RegisterJobs registers auth email job handlers with the queue.
// Skipped when no email service is configured (WithEmailService was not called),
// since there is nothing to deliver.
func (a *App[P]) RegisterJobs(q burrow.Queue) {
	if a.emailService == nil {
		return
	}
	a.emailTask = burrow.DefineTask("auth.send_email",
		a.handleEmailJob, burrow.WithMaxRetries(5))
	a.emailTask.Register(q)
}

// handleEmailJob processes an email delivery job.
func (a *App[P]) handleEmailJob(ctx context.Context, p emailJobPayload) error {
	ctx = a.withLocale(ctx, p.Locale)

	switch p.Kind {
	case "verification":
		return a.emailService.SendVerification(ctx, p.Email, p.URL)
	case "invite":
		return a.emailService.SendInvite(ctx, p.Email, p.URL)
	default:
		return fmt.Errorf("unknown email kind: %q", p.Kind)
	}
}

// enqueueEmail enqueues an email delivery job. If no task is configured,
// it falls back to sending the email directly (synchronous).
func (a *App[P]) enqueueEmail(ctx context.Context, kind, email, url string) error {
	if a.emailTask == nil {
		return a.sendEmailDirect(ctx, kind, email, url)
	}
	_, err := a.emailTask.Enqueue(ctx, emailJobPayload{
		Kind:   kind,
		Email:  email,
		URL:    url,
		Locale: i18n.Locale(ctx),
	})
	return err
}

// sendEmailDirect sends an email synchronously (fallback when no queue).
func (a *App[P]) sendEmailDirect(ctx context.Context, kind, email, url string) error {
	if a.emailService == nil {
		return nil
	}
	switch kind {
	case "verification":
		return a.emailService.SendVerification(ctx, email, url)
	case "invite":
		return a.emailService.SendInvite(ctx, email, url)
	default:
		return fmt.Errorf("unknown email kind: %q", kind)
	}
}
