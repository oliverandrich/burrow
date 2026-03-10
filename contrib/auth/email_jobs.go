package auth

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/i18n"
)

// emailJobPayload is the JSON payload for the auth.send_email job.
type emailJobPayload struct {
	Kind   string `json:"kind"` // "verification" or "invite"
	Email  string `json:"email"`
	URL    string `json:"url"`
	Locale string `json:"locale"`
}

// RegisterJobs registers auth email job handlers with the queue.
func (a *App) RegisterJobs(q burrow.Queue) {
	if a.emailService == nil {
		return
	}
	q.Handle("auth.send_email", a.handleEmailJob, burrow.WithMaxRetries(5))
	a.jobs = q
}

// handleEmailJob processes an email delivery job.
func (a *App) handleEmailJob(ctx context.Context, payload []byte) error {
	var p emailJobPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("unmarshal email job payload: %w", err)
	}

	ctx = a.i18nApp.WithLocale(ctx, p.Locale)

	switch p.Kind {
	case "verification":
		return a.emailService.SendVerification(ctx, p.Email, p.URL)
	case "invite":
		return a.emailService.SendInvite(ctx, p.Email, p.URL)
	default:
		return fmt.Errorf("unknown email kind: %q", p.Kind)
	}
}

// enqueueEmail enqueues an email delivery job. If no queue is configured,
// it falls back to sending the email directly (synchronous).
func (a *App) enqueueEmail(ctx context.Context, kind, email, url string) error {
	if a.jobs == nil {
		return a.sendEmailDirect(ctx, kind, email, url)
	}
	_, err := a.jobs.Enqueue(ctx, "auth.send_email", emailJobPayload{
		Kind:   kind,
		Email:  email,
		URL:    url,
		Locale: i18n.Locale(ctx),
	})
	return err
}

// sendEmailDirect sends an email synchronously (fallback when no queue).
func (a *App) sendEmailDirect(ctx context.Context, kind, email, url string) error {
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
