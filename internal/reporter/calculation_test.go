package reporter

import (
	"math"
	"testing"
	"time"
)

func TestWindowCalendarBoundaries(t *testing.T) {
	zone, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		instant string
		period  string
		hours   float64
	}{
		{"2024-04-01T08:00:00+02:00", "daily", 23},
		{"2024-10-28T08:00:00+01:00", "daily", 25},
		{"2024-03-04T08:00:00+01:00", "weekly", 168},
		{"2024-03-01T08:00:00+01:00", "monthly", 29 * 24},
	}
	for _, tc := range tests {
		instant, err := time.Parse(time.RFC3339, tc.instant)
		if err != nil {
			t.Fatal(err)
		}
		start, end, err := Window(instant, zone, tc.period)
		if err != nil {
			t.Fatal(err)
		}
		if got := end.Sub(start).Hours(); got != tc.hours {
			t.Errorf("%s %s: got %v hours, want %v", tc.instant, tc.period, got, tc.hours)
		}
	}
}

func TestIntegrateGapsAndDuplicates(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	samples := []Sample{{start, 1000}, {start, 1000}, {start.Add(time.Minute), math.NaN()}, {start.Add(2 * time.Minute), 2000}}
	result, err := Integrate(start, start.Add(3*time.Minute), time.Minute, samples)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(result.EnergyKWh-0.05) > 1e-10 || result.Covered != 2*time.Minute || result.PeakWatts != 2000 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if avg, ok := result.AverageWatts(); !ok || avg != 1500 {
		t.Fatalf("average = %v, %v", avg, ok)
	}
}
