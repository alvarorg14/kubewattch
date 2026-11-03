package collector

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type testReader struct {
	value Observation
	err   error
}

func (r testReader) Read(context.Context) (Observation, error) { return r.value, r.err }

func TestMetricsOmitFailedAndStaleObservations(t *testing.T) {
	m := NewMetrics("test", "plug", "tapo", time.Second)
	registry := prometheus.NewRegistry()
	registry.MustRegister(m)
	zero := 0.0
	if err := m.PollOnce(context.Background(), testReader{value: Observation{Time: time.Now(), PowerWatts: &zero}}); err != nil {
		t.Fatal(err)
	}
	gather := func() map[string]bool {
		t.Helper()
		families, err := registry.Gather()
		if err != nil {
			t.Fatal(err)
		}
		names := make(map[string]bool)
		for _, f := range families {
			names[f.GetName()] = true
		}
		return names
	}
	if !gather()["kubewattch_device_power_watts"] {
		t.Fatal("valid zero was omitted")
	}
	if err := m.PollOnce(context.Background(), testReader{err: errors.New("offline")}); err == nil {
		t.Fatal("expected failure")
	}
	names := gather()
	if names["kubewattch_device_power_watts"] || !names["kubewattch_device_collection_errors_total"] || !names["kubewattch_device_last_success_timestamp_seconds"] {
		t.Fatalf("unexpected metrics after failure: %v", names)
	}
	if err := m.PollOnce(context.Background(), testReader{value: Observation{Time: time.Now().Add(-3 * time.Second), PowerWatts: &zero}}); err != nil {
		t.Fatal(err)
	}
	if gather()["kubewattch_device_power_watts"] {
		t.Fatal("stale reading was exported")
	}
}
