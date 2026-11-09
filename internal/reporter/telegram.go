package reporter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Sender delivers report messages to a configured destination.
type Sender interface {
	Send(context.Context, string) error
}

// Telegram sends HTML messages using a Telegram bot. Credentials must never be
// included in errors, including transport errors containing the request URL.
type Telegram struct {
	Token   string
	ChatID  string
	Client  *http.Client
	BaseURL string
}

func (t Telegram) Send(ctx context.Context, message string) error {
	if t.Token == "" || t.ChatID == "" || message == "" {
		return errors.New("telegram configuration or message is empty")
	}
	base := t.BaseURL
	if base == "" {
		base = "https://api.telegram.org"
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.User != nil {
		return errors.New("invalid telegram endpoint")
	}
	payload, err := json.Marshal(map[string]string{"chat_id": t.ChatID, "text": message, "parse_mode": "HTML"})
	if err != nil {
		return errors.New("telegram message encoding failed")
	}
	client := t.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	// Clone the client to disallow redirects, including redirects to the same host.
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	endpoint := base + "/bot" + url.PathEscape(t.Token) + "/sendMessage"
	for attempt := 0; attempt < 3; attempt++ {
		req, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if requestErr != nil {
			return errors.New("telegram request failed")
		}
		req.Header.Set("Content-Type", "application/json")
		resp, requestErr := copyClient.Do(req)
		delay := time.Duration(attempt+1) * time.Second
		if requestErr == nil {
			var response struct {
				OK         bool `json:"ok"`
				Parameters struct {
					RetryAfter int `json:"retry_after"`
				} `json:"parameters"`
			}
			decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&response)
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 && decodeErr == nil && response.OK {
				return nil
			}
			retry := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
			if response.Parameters.RetryAfter > 0 {
				delay = time.Duration(response.Parameters.RetryAfter) * time.Second
			}
			if header := resp.Header.Get("Retry-After"); header != "" {
				if seconds, parseErr := strconv.Atoi(header); parseErr == nil && seconds > 0 {
					delay = time.Duration(seconds) * time.Second
				}
			}
			if !retry {
				return fmt.Errorf("telegram delivery failed (HTTP %d)", resp.StatusCode)
			}
		}
		if attempt == 2 {
			break
		}
		// The Job deadline or caller cancellation bounds long rate-limit waits.
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.New("telegram delivery canceled")
		case <-timer.C:
		}
	}
	return errors.New("telegram delivery failed after retries")
}
