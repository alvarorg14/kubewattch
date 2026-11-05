package reporter

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPrometheusPower(t *testing.T) {
	start := time.Unix(1704067200, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fake-token" {
			t.Error("missing token")
		}
		query := r.URL.Query().Get("query")
		if !strings.Contains(query, `device_namespace="test"`) || !strings.Contains(query, "collection_success") {
			t.Errorf("unexpected query %q", query)
		}
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"matrix","result":[{"values":[[%d,"1000"],[%d,"NaN"]]}]}}`, start.Unix(), start.Add(time.Minute).Unix())
	}))
	defer server.Close()
	samples, err := (Prometheus{URL: server.URL, BearerToken: "fake-token"}).Power(context.Background(), "test", "device", 30*time.Second, start, start.Add(2*time.Minute))
	if err != nil || len(samples) != 1 || samples[0].Watts != 1000 {
		t.Fatalf("samples=%v error=%v", samples, err)
	}
}

func TestPrometheusErrorsAreSanitized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "fake-token", http.StatusForbidden) }))
	defer server.Close()
	start := time.Unix(1704067200, 0)
	_, err := (Prometheus{URL: server.URL, BearerToken: "fake-token"}).Power(context.Background(), "test", "device", time.Minute, start, start.Add(time.Minute))
	if err == nil || strings.Contains(err.Error(), "fake-token") {
		t.Fatalf("unexpected error: %v", err)
	}
}
