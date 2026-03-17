package alerts

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// sendTelegram sends a Markdown message via the Telegram Bot API
func sendTelegram(token, chatID, text string) error {
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)

	resp, err := (&http.Client{Timeout: 10 * time.Second}).PostForm(endpoint, url.Values{
		"chat_id":    {chatID},
		"text":       {text},
		"parse_mode": {"Markdown"},
	})
	if err != nil {
		return fmt.Errorf("telegram send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram send: status %d", resp.StatusCode)
	}
	return nil
}
