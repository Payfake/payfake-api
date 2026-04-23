package service

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/payfake/payfake-api/internal/domain"
)

type fakeWebhookRepo struct {
	attempts       []*domain.WebhookAttempt
	recordCalls    int
	lastEventID    string
	lastDelivered  bool
	findEvent      *domain.WebhookEvent
	findEventErr   error
	listEvents     []domain.WebhookEvent
	listTotal      int64
	listErr        error
	attemptList    []domain.WebhookAttempt
	attemptListErr error
}

func (f *fakeWebhookRepo) CreateEvent(*domain.WebhookEvent) error { return nil }

func (f *fakeWebhookRepo) CreateAttempt(attempt *domain.WebhookAttempt) error {
	f.attempts = append(f.attempts, attempt)
	return nil
}

func (f *fakeWebhookRepo) RecordAttemptResult(id string, delivered bool) error {
	f.recordCalls++
	f.lastEventID = id
	f.lastDelivered = delivered
	return nil
}

func (f *fakeWebhookRepo) FindEventByID(string, string) (*domain.WebhookEvent, error) {
	return f.findEvent, f.findEventErr
}

func (f *fakeWebhookRepo) ListEvents(string, int, int) ([]domain.WebhookEvent, int64, error) {
	return f.listEvents, f.listTotal, f.listErr
}

func (f *fakeWebhookRepo) GetAttempts(string) ([]domain.WebhookAttempt, error) {
	return f.attemptList, f.attemptListErr
}

func (f *fakeWebhookRepo) FindUndeliveredEvents() ([]domain.WebhookEvent, error) {
	return f.listEvents, f.listErr
}

type fakeMerchantRepo struct {
	merchant *domain.Merchant
	err      error
}

func (f *fakeMerchantRepo) FindByID(string) (*domain.Merchant, error) {
	return f.merchant, f.err
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestWebhookServiceDeliverCapturesResponseBodyAndAttemptResult(t *testing.T) {
	webhookRepo := &fakeWebhookRepo{}
	merchantRepo := &fakeMerchantRepo{
		merchant: &domain.Merchant{
			Base:       domain.Base{ID: "MRC_123"},
			SecretKey:  "sk_test_123",
			WebhookURL: "https://merchant.example/webhook",
		},
	}
	svc := &WebhookService{
		webhookRepo:  webhookRepo,
		merchantRepo: merchantRepo,
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(strings.NewReader("downstream unavailable")),
				Header:     make(http.Header),
			}, nil
		})},
	}

	svc.deliver(&domain.WebhookEvent{Base: domain.Base{ID: "WHK_123"}, MerchantID: "MRC_123"}, []byte(`{"event":"charge.failed"}`))

	if len(webhookRepo.attempts) != 1 {
		t.Fatalf("expected one attempt, got %d", len(webhookRepo.attempts))
	}
	attempt := webhookRepo.attempts[0]
	if attempt.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected status code 502, got %d", attempt.StatusCode)
	}
	if attempt.ResponseBody != "downstream unavailable" {
		t.Fatalf("expected response body to be captured, got %q", attempt.ResponseBody)
	}
	if webhookRepo.recordCalls != 1 || webhookRepo.lastEventID != "WHK_123" || webhookRepo.lastDelivered {
		t.Fatalf("expected one failed record update, got calls=%d event=%q delivered=%v", webhookRepo.recordCalls, webhookRepo.lastEventID, webhookRepo.lastDelivered)
	}
}

func TestWebhookServiceDeliverMarksSuccessfulAttempt(t *testing.T) {
	webhookRepo := &fakeWebhookRepo{}
	merchantRepo := &fakeMerchantRepo{
		merchant: &domain.Merchant{
			Base:       domain.Base{ID: "MRC_123"},
			SecretKey:  "sk_test_123",
			WebhookURL: "https://merchant.example/webhook",
		},
	}
	svc := &WebhookService{
		webhookRepo:  webhookRepo,
		merchantRepo: merchantRepo,
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("ok")),
				Header:     make(http.Header),
			}, nil
		})},
	}

	svc.deliver(&domain.WebhookEvent{Base: domain.Base{ID: "WHK_456"}, MerchantID: "MRC_123"}, []byte(`{"event":"charge.success"}`))

	if len(webhookRepo.attempts) != 1 {
		t.Fatalf("expected one attempt, got %d", len(webhookRepo.attempts))
	}
	if !webhookRepo.attempts[0].Succeeded {
		t.Fatal("expected successful attempt")
	}
	if webhookRepo.recordCalls != 1 || !webhookRepo.lastDelivered {
		t.Fatalf("expected successful record update, got calls=%d delivered=%v", webhookRepo.recordCalls, webhookRepo.lastDelivered)
	}
}
