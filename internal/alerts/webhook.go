package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type webhookPayload struct {
	Subject   string `json:"subject"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	Source    string `json:"source"`
}

// sendWebhook POSTs a JSON payload to the given URL
func sendWebhook(url string, subject, message string) error {
	body, err := json.Marshal(webhookPayload{
		Subject:   subject,
		Message:   message,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Source:    "9level-collector",
	})
	if err != nil {
		return fmt.Errorf("webhook marshal: %w", err)
	}

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook send: status %d", resp.StatusCode)
	}
	return nil
}
