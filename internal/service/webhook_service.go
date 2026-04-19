package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/GordenArcher/payfake/internal/domain"
	"github.com/GordenArcher/payfake/internal/repository"
	"github.com/GordenArcher/payfake/pkg/crypto"
	"github.com/GordenArcher/payfake/pkg/uid"
)

type WebhookService struct {
	webhookRepo  *repository.WebhookRepository
	merchantRepo *repository.MerchantRepository
}

func NewWebhookService(
	webhookRepo *repository.WebhookRepository,
	merchantRepo *repository.MerchantRepository,
) *WebhookService {
	return &WebhookService{
		webhookRepo:  webhookRepo,
		merchantRepo: merchantRepo,
	}
}

// Dispatch creates a webhook event record and immediately attempts delivery.
// We record the event BEFORE attempting delivery, if the delivery
// goroutine panics or the server restarts we still have a record
// of what needs to be delivered and can retry from the control panel.
func (s *WebhookService) Dispatch(
	merchantID string,
	transactionID string,
	eventType domain.WebhookEventType,
	transaction *domain.Transaction,
) error {
	// Build the webhook payload, same shape as Paystack's webhook body.
	// Developers verify our webhooks the same way they verify Paystack's.
	payload := map[string]any{
		"event": string(eventType),
		"data":  transaction,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	event := &domain.WebhookEvent{
		Base:          domain.Base{ID: uid.NewWebhookEventID()},
		MerchantID:    merchantID,
		TransactionID: transactionID,
		Event:         eventType,
		Payload:       domain.JSON(payload),
		Delivered:     false,
		Attempts:      0,
	}

	if err := s.webhookRepo.CreateEvent(event); err != nil {
		return fmt.Errorf("failed to create webhook event: %w", err)
	}

	// Attempt delivery immediately in a goroutine so the charge
	// response is not blocked by the webhook HTTP call.
	// The developer's webhook endpoint could be slow or unreachable,
	// we don't want that to affect the charge response time.
	go s.deliver(event, payloadBytes)

	return nil
}

// deliver attempts to POST the webhook payload to the merchant's endpoint.
// It records the attempt regardless of success or failure so developers
// can inspect exactly what was sent and what response they returned.
func (s *WebhookService) deliver(event *domain.WebhookEvent, payloadBytes []byte) {
	// Look up the merchant to get their webhook URL and secret key.
	merchant, err := s.merchantRepo.FindByID(event.MerchantID)
	if err != nil || merchant.WebhookURL == "" {
		// No webhook URL configured, nothing to deliver to.
		// We still record the attempt so it's visible in the dashboard.
		return
	}

	req, err := http.NewRequest("POST", merchant.WebhookURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")

	// Sign the payload with the merchant's secret key.
	// The developer verifies this signature to confirm the request
	// genuinely came from Payfake and wasn't tampered with in transit.
	signature := crypto.Sign(merchant.SecretKey, payloadBytes)
	req.Header.Set(crypto.SignatureHeader, signature)

	// 10 second timeout, we don't want a slow merchant endpoint
	// to keep the goroutine alive indefinitely.
	client := &http.Client{Timeout: 10 * time.Second}

	attemptedAt := time.Now()
	resp, err := client.Do(req)

	attempt := &domain.WebhookAttempt{
		Base:           domain.Base{ID: uid.NewWebhookAttemptID()},
		WebhookEventID: event.ID,
		AttemptedAt:    attemptedAt,
	}

	if err != nil {
		// Network error, couldn't reach the endpoint.
		attempt.StatusCode = 0
		attempt.ResponseBody = err.Error()
		attempt.Succeeded = false
	} else {
		defer resp.Body.Close()
		attempt.StatusCode = resp.StatusCode
		// A 2xx response from the merchant's endpoint means they
		// received and acknowledged the webhook successfully.
		attempt.Succeeded = resp.StatusCode >= 200 && resp.StatusCode < 300
	}

	s.webhookRepo.CreateAttempt(attempt)

	newAttempts := event.Attempts + 1
	delivered := attempt.Succeeded
	s.webhookRepo.UpdateEventDelivery(event.ID, delivered, newAttempts)
}

// Retry manually re-triggers delivery for a specific webhook event.
// Called from the control panel when a developer wants to re-send
// a webhook that failed delivery.
func (s *WebhookService) Retry(id, merchantID string) error {
	event, err := s.webhookRepo.FindEventByID(id, merchantID)
	if err != nil {
		return ErrWebhookNotFound
	}

	payloadBytes, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload for retry: %w", err)
	}

	go s.deliver(event, payloadBytes)
	return nil
}

// List returns paginated webhook events for a merchant.
func (s *WebhookService) List(merchantID string, page, perPage int) ([]domain.WebhookEvent, int64, error) {
	offset := (page - 1) * perPage
	return s.webhookRepo.ListEvents(merchantID, offset, perPage)
}

// GetAttempts returns all delivery attempts for a webhook event.
func (s *WebhookService) GetAttempts(id, merchantID string) ([]domain.WebhookAttempt, error) {
	_, err := s.webhookRepo.FindEventByID(id, merchantID)
	if err != nil {
		return nil, ErrWebhookNotFound
	}
	return s.webhookRepo.GetAttempts(id)
}

// StartRetryWorker launches a background goroutine that periodically
// retries undelivered webhook events. It runs every 60 seconds and
// picks up any events that failed delivery and have fewer than 3 attempts.
// This prevents webhooks from being silently lost when the merchant's
// endpoint is temporarily down.
// Call this once from main.go after the server starts.
func (s *WebhookService) StartRetryWorker(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		log.Println("[payfake] webhook retry worker started")

		for {
			select {
			case <-ticker.C:
				s.retryUndelivered()
			case <-ctx.Done():
				// Context cancelled, server is shutting down.
				// Stop the worker cleanly so we don't retry during shutdown.
				log.Println("[payfake] webhook retry worker stopped")
				return
			}
		}
	}()
}

// retryUndelivered finds all undelivered webhook events with fewer
// than 3 attempts and re-triggers delivery for each one.
func (s *WebhookService) retryUndelivered() {
	events, err := s.webhookRepo.FindUndeliveredEvents()
	if err != nil {
		log.Printf("[payfake] retry worker: failed to fetch undelivered events: %v", err)
		return
	}

	if len(events) == 0 {
		return
	}

	log.Printf("[payfake] retry worker: retrying %d undelivered webhook(s)", len(events))

	for _, event := range events {
		payloadBytes, err := json.Marshal(event.Payload)
		if err != nil {
			continue
		}
		// Deliver in a goroutine so one slow endpoint doesn't block the others.
		go s.deliver(&event, payloadBytes)
	}
}
