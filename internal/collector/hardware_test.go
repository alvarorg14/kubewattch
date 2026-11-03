package collector

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestTapoHardware performs read-only requests and is never enabled by default.
func TestTapoHardware(t *testing.T) {
	if os.Getenv("KUBEWATTCH_HARDWARE_TEST") != "1" {
		t.Skip("set KUBEWATTCH_HARDWARE_TEST=1 for the live device test")
	}
	address, email, password := os.Getenv("TAPO_HOST"), os.Getenv("TAPO_EMAIL"), os.Getenv("TAPO_PASSWORD")
	if address == "" || email == "" || password == "" {
		t.Fatal("TAPO_HOST, TAPO_EMAIL and TAPO_PASSWORD must be set")
	}
	reader := NewTapoReader(address, email, password)
	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		obs, err := reader.Read(ctx)
		cancel()
		if err != nil {
			t.Fatal("live device collection failed")
		}
		if obs.PowerWatts == nil || obs.TodayJoules == nil || obs.MonthJoules == nil {
			t.Fatal("live device returned incomplete measurements")
		}
	}
}
