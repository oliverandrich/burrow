package auth

import (
	"context"
	"testing"
	"time"

	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockQueue implements burrow.Queue for testing.
type mockQueue struct { //nolint:govet // fieldalignment: test struct, readability preferred
	handlers  map[string]burrow.JobHandlerFunc
	enqueued  []mockEnqueuedJob
	dequeued  []string
	enqueueFn func(typeName string, payload any) (string, error)
}

type mockEnqueuedJob struct { //nolint:govet // fieldalignment: test struct, readability preferred
	typeName string
	payload  any
}

func newMockQueue() *mockQueue {
	return &mockQueue{
		handlers: make(map[string]burrow.JobHandlerFunc),
	}
}

func (q *mockQueue) Handle(typeName string, fn burrow.JobHandlerFunc, _ ...burrow.JobOption) {
	q.handlers[typeName] = fn
}

func (q *mockQueue) Enqueue(_ context.Context, typeName string, payload any) (string, error) {
	if q.enqueueFn != nil {
		return q.enqueueFn(typeName, payload)
	}
	q.enqueued = append(q.enqueued, mockEnqueuedJob{typeName: typeName, payload: payload})
	return "job-1", nil
}

func (q *mockQueue) EnqueueAt(_ context.Context, typeName string, payload any, _ time.Time) (string, error) {
	q.enqueued = append(q.enqueued, mockEnqueuedJob{typeName: typeName, payload: payload})
	return "job-1", nil
}

func (q *mockQueue) Dequeue(_ context.Context, id string) error {
	q.dequeued = append(q.dequeued, id)
	return nil
}

// Compile-time check that App implements HasJobs.
var _ burrow.HasJobs = (*App[EmptyProfile])(nil)

func TestRegisterJobs(t *testing.T) {
	q := newMockQueue()
	app := &App[EmptyProfile]{emailService: &mockEmailService{}}
	app.RegisterJobs(q)

	assert.Contains(t, q.handlers, "auth.send_email")
	assert.NotNil(t, app.emailTask)
}

func TestRegisterJobs_NoEmailService(t *testing.T) {
	q := newMockQueue()
	app := &App[EmptyProfile]{}
	app.RegisterJobs(q)

	// No handler registered when email service is not configured.
	assert.Empty(t, q.handlers)
	assert.Nil(t, app.emailTask)
}

func TestHandleEmailJob_Verification(t *testing.T) {
	emailSvc := &mockEmailService{}
	bundle := testI18nBundle(t)
	app := &App[EmptyProfile]{emailService: emailSvc, withLocale: bundle.WithLocale}

	err := app.handleEmailJob(context.Background(), emailJobPayload{
		Kind:   "verification",
		Email:  "test@example.com",
		URL:    "http://localhost/verify",
		Locale: "en",
	})
	require.NoError(t, err)
	assert.True(t, emailSvc.sendCalled)
}

func TestHandleEmailJob_Invite(t *testing.T) {
	emailSvc := &mockEmailService{}
	bundle := testI18nBundle(t)
	app := &App[EmptyProfile]{emailService: emailSvc, withLocale: bundle.WithLocale}

	err := app.handleEmailJob(context.Background(), emailJobPayload{
		Kind:   "invite",
		Email:  "invitee@example.com",
		URL:    "http://localhost/register?invite=abc",
		Locale: "en",
	})
	require.NoError(t, err)
	assert.True(t, emailSvc.sendCalled)
}

func TestHandleEmailJob_UnknownKind(t *testing.T) {
	emailSvc := &mockEmailService{}
	bundle := testI18nBundle(t)
	app := &App[EmptyProfile]{emailService: emailSvc, withLocale: bundle.WithLocale}

	err := app.handleEmailJob(context.Background(), emailJobPayload{
		Kind:   "unknown",
		Email:  "test@example.com",
		URL:    "http://localhost",
		Locale: "en",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown email kind")
}

func TestEnqueueEmail(t *testing.T) {
	q := newMockQueue()
	bundle := testI18nBundle(t)
	app := &App[EmptyProfile]{emailService: &mockEmailService{}, withLocale: bundle.WithLocale}
	app.RegisterJobs(q)

	err := app.enqueueEmail(context.Background(), "verification", "test@example.com", "http://localhost/verify")
	require.NoError(t, err)

	require.Len(t, q.enqueued, 1)
	assert.Equal(t, "auth.send_email", q.enqueued[0].typeName)

	p, ok := q.enqueued[0].payload.(emailJobPayload)
	require.True(t, ok)
	assert.Equal(t, "verification", p.Kind)
	assert.Equal(t, "test@example.com", p.Email)
}

func TestEnqueueEmail_FallbackDirect(t *testing.T) {
	emailSvc := &mockEmailService{}
	bundle := testI18nBundle(t)
	app := &App[EmptyProfile]{emailService: emailSvc, withLocale: bundle.WithLocale} // no emailTask

	err := app.enqueueEmail(context.Background(), "verification", "test@example.com", "http://localhost/verify")
	require.NoError(t, err)
	assert.True(t, emailSvc.sendCalled)
}

// --- sendEmailDirect additional paths ---

func TestSendEmailDirectInvite(t *testing.T) {
	emailSvc := &mockEmailService{}
	bundle := testI18nBundle(t)
	app := &App[EmptyProfile]{emailService: emailSvc, withLocale: bundle.WithLocale}

	err := app.sendEmailDirect(context.Background(), "invite", "test@example.com", "http://localhost/register")
	require.NoError(t, err)
	assert.True(t, emailSvc.sendCalled)
}

func TestSendEmailDirectUnknownKind(t *testing.T) {
	emailSvc := &mockEmailService{}
	bundle := testI18nBundle(t)
	app := &App[EmptyProfile]{emailService: emailSvc, withLocale: bundle.WithLocale}

	err := app.sendEmailDirect(context.Background(), "unknown", "test@example.com", "http://localhost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown email kind")
}

func TestSendEmailDirectNilService(t *testing.T) {
	app := &App[EmptyProfile]{}

	err := app.sendEmailDirect(context.Background(), "verification", "test@example.com", "http://localhost")
	require.NoError(t, err)
}
