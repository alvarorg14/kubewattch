package main

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alvarorg14/kubewattch/internal/collector"
)

func TestCollectorAgainstSimulatedTapo(t *testing.T) {
	server := httptest.NewServer(&simulator{})
	defer server.Close()
	reader := collector.NewTapoReader(strings.TrimPrefix(server.URL, "http://"), email, password)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reading, err := reader.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if reading.PowerWatts == nil || *reading.PowerWatts != 12.5 || reading.TodayJoules == nil || *reading.TodayJoules != 3600000 {
		t.Fatalf("unexpected reading: %+v", reading)
	}
}
