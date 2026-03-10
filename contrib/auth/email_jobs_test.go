package auth

import (
	"context"
	"encoding/json"
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
var _ burrow.HasJobs = (*App)(nil)

func TestRegisterJobs(t *testing.T) {
	q := newMockQueue()
	app := &App{emailService: &mockEmailService{}}
	app.RegisterJobs(q)

	assert.Contains(t, q.handlers, "auth.send_email")
	assert.Same(t, q, app.jobs)
}

func TestRegisterJobs_NoEmailService(t *testing.T) {
	q := newMockQueue()
	app := &App{}
	app.RegisterJobs(q)

	assert.Empty(t, q.handlers)
	assert.Nil(t, app.jobs)
}

func TestHandleEmailJob_Verification(t *testing.T) {
	emailSvc := &mockEmailService{}
	bundle := testI18nBundle(t)
	app := &App{emailService: emailSvc, withLocale: bundle.WithLocale}

	payload, err := json.Marshal(emailJobPayload{
		Kind:   "verification",
		Email:  "test@example.com",
		URL:    "http://localhost/verify",
		Locale: "en",
	})
	require.NoError(t, err)

	err = app.handleEmailJob(context.Background(), payload)
	require.NoError(t, err)
	assert.True(t, emailSvc.sendCalled)
}

func TestHandleEmailJob_Invite(t *testing.T) {
	emailSvc := &mockEmailService{}
	bundle := testI18nBundle(t)
	app := &App{emailService: emailSvc, withLocale: bundle.WithLocale}

	payload, err := json.Marshal(emailJobPayload{
		Kind:   "invite",
		Email:  "invitee@example.com",
		URL:    "http://localhost/register?invite=abc",
		Locale: "en",
	})
	require.NoError(t, err)

	err = app.handleEmailJob(context.Background(), payload)
	require.NoError(t, err)
	assert.True(t, emailSvc.sendCalled)
}

func TestHandleEmailJob_UnknownKind(t *testing.T) {
	emailSvc := &mockEmailService{}
	bundle := testI18nBundle(t)
	app := &App{emailService: emailSvc, withLocale: bundle.WithLocale}

	payload, err := json.Marshal(emailJobPayload{
		Kind:   "unknown",
		Email:  "test@example.com",
		URL:    "http://localhost",
		Locale: "en",
	})
	require.NoError(t, err)

	err = app.handleEmailJob(context.Background(), payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown email kind")
}

func TestHandleEmailJob_InvalidPayload(t *testing.T) {
	emailSvc := &mockEmailService{}
	bundle := testI18nBundle(t)
	app := &App{emailService: emailSvc, withLocale: bundle.WithLocale}

	err := app.handleEmailJob(context.Background(), []byte("not json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal email job payload")
}

func TestEnqueueEmail(t *testing.T) {
	q := newMockQueue()
	bundle := testI18nBundle(t)
	app := &App{jobs: q, emailService: &mockEmailService{}, withLocale: bundle.WithLocale}

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
	app := &App{emailService: emailSvc, withLocale: bundle.WithLocale} // no jobs queue

	err := app.enqueueEmail(context.Background(), "verification", "test@example.com", "http://localhost/verify")
	require.NoError(t, err)
	assert.True(t, emailSvc.sendCalled)
}

// --- sendEmailDirect additional paths ---

func TestSendEmailDirectInvite(t *testing.T) {
	emailSvc := &mockEmailService{}
	bundle := testI18nBundle(t)
	app := &App{emailService: emailSvc, withLocale: bundle.WithLocale}

	err := app.sendEmailDirect(context.Background(), "invite", "test@example.com", "http://localhost/register")
	require.NoError(t, err)
	assert.True(t, emailSvc.sendCalled)
}

func TestSendEmailDirectUnknownKind(t *testing.T) {
	emailSvc := &mockEmailService{}
	bundle := testI18nBundle(t)
	app := &App{emailService: emailSvc, withLocale: bundle.WithLocale}

	err := app.sendEmailDirect(context.Background(), "unknown", "test@example.com", "http://localhost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown email kind")
}

func TestSendEmailDirectNilService(t *testing.T) {
	app := &App{}

	err := app.sendEmailDirect(context.Background(), "verification", "test@example.com", "http://localhost")
	require.NoError(t, err)
}
