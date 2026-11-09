package reporter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTelegramRedactsTokenAndRejectsRedirect(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("redirect was followed") }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer server.Close()
	sender := Telegram{Token: "synthetic-secret", ChatID: "synthetic-chat", BaseURL: server.URL}
	err := sender.Send(context.Background(), "test")
	if err == nil || strings.Contains(err.Error(), sender.Token) || strings.Contains(err.Error(), sender.ChatID) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTelegramSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Error("expected POST")
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	if err := (Telegram{Token: "fake", ChatID: "fake", BaseURL: server.URL}).Send(context.Background(), "report"); err != nil {
		t.Fatal(err)
	}
}

func TestTelegramRateLimitWaitsBeforeRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"ok":false,"parameters":{"retry_after":1}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	start := time.Now()
	if err := (Telegram{Token: "fake", ChatID: "fake", BaseURL: server.URL}).Send(context.Background(), "report"); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || time.Since(start) < time.Second {
		t.Fatalf("rate limit ignored: %d attempts in %s", attempts, time.Since(start))
	}
}
