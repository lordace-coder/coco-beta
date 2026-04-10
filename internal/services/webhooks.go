package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// WebhookEvent is the payload sent to webhook URLs.
type WebhookEvent struct {
	Event      string                 `json:"event"`       // "pre_save" | "post_save" | "pre_delete" | "post_delete"
	Collection string                 `json:"collection"`  // collection ID
	DocumentID string                 `json:"document_id"` // may be empty for pre_save on create
	Data       map[string]interface{} `json:"data"`
	Timestamp  string                 `json:"timestamp"`
}

var webhookClient = &http.Client{Timeout: 10 * time.Second}

// FireWebhook sends a WebhookEvent to a URL in a fire-and-forget goroutine.
// Errors are logged but not returned — webhooks must never block a request.
func FireWebhook(url, event, collectionID, documentID string, data map[string]interface{}) {
	if url == "" {
		return
	}
	go func() {
		payload := WebhookEvent{
			Event:      event,
			Collection: collectionID,
			DocumentID: documentID,
			Data:       data,
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
		}
		body, err := json.Marshal(payload)
		if err != nil {
			log.Printf("webhook: marshal error for %s: %v", url, err)
			return
		}
		req, err := http.NewRequest("POST", url, bytes.NewReader(body))
		if err != nil {
			log.Printf("webhook: bad URL %s: %v", url, err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Cocobase/1.0")

		resp, err := webhookClient.Do(req)
		if err != nil {
			log.Printf("webhook: request to %s failed: %v", url, err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			log.Printf("webhook: %s responded %d for event %s", url, resp.StatusCode, event)
			return
		}
		log.Printf("webhook: fired %s → %s (%d)", event, url, resp.StatusCode)
	}()
}

// FireWebhookSync is the same but blocks — used for pre_* hooks where callers
// may want to inspect the response in the future (currently still fire-and-forget
// at the business level, but kept synchronous for ordering guarantees).
func FireWebhookSync(url, event, collectionID, documentID string, data map[string]interface{}) error {
	if url == "" {
		return nil
	}
	payload := WebhookEvent{
		Event:      event,
		Collection: collectionID,
		DocumentID: documentID,
		Data:       data,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("webhook: marshal: %w", err)
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook: bad URL: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Cocobase/1.0")

	resp, err := webhookClient.Do(req)
	if err != nil {
		return fmt.Errorf("webhook: request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook: remote returned %d", resp.StatusCode)
	}
	return nil
}
