package reporter

import (
	"context"
	"strings"
	"testing"
	"time"
)

type fixedSource struct{ missing string }

func (f fixedSource) Power(_ context.Context, _, device string, _ time.Duration, start, end time.Time) ([]Sample, error) {
	if device == f.missing {
		return nil, nil
	}
	var samples []Sample
	for at := start; at.Before(end); at = at.Add(time.Minute) {
		samples = append(samples, Sample{Time: at, Watts: 1000})
	}
	return samples, nil
}
func TestBuildReportsCoverageAndEscapesNames(t *testing.T) {
	scheduled := time.Date(2024, 3, 2, 8, 0, 0, 0, time.UTC)
	in := ReportInput{Name: "<report>", Namespace: "ns", Period: "daily", Currency: "EUR", Price: "0.15", Scheduled: scheduled, Location: time.UTC, Devices: []Device{{Name: "one", Interval: time.Minute}, {Name: "two", Interval: time.Minute}}}
	report, err := Build(context.Background(), fixedSource{missing: "two"}, in)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(report.Chunks, "\n")
	for _, want := range []string{"&lt;report&gt;", "Total (partial)</b>: 24.000 kWh", "coverage 0%", "Projections: N/A", "simultaneous average N/A"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in %s", want, text)
		}
	}
	if report.Complete {
		t.Error("incomplete report marked complete")
	}
}
func TestSplitLinesKeepsEntitiesAndTags(t *testing.T) {
	chunks, err := splitLines([]string{"<b>hello &amp;</b>", "<b>world</b>"}, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 2 {
		t.Fatalf("unexpected chunks: %v", chunks)
	}
}

func TestBuildProjectsAndUsesSimultaneousPeak(t *testing.T) {
	scheduled := time.Date(2024, 2, 3, 8, 0, 0, 0, time.UTC)
	in := ReportInput{Name: "report", Namespace: "ns", Period: "daily", Currency: "EUR", Price: "0.15", Scheduled: scheduled, Location: time.UTC, Devices: []Device{{Name: "one", Interval: time.Minute}, {Name: "two", Interval: time.Minute}}}
	report, err := Build(context.Background(), fixedSource{}, in)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(report.Chunks, "\n")
	for _, want := range []string{"48.000 kWh", "simultaneous average 2000.0 W", "sampled peak 2000.0 W", "Projected month: 1392.000 kWh (EUR 208.80)", "annualized cost: EUR 2635.20"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in %s", want, text)
		}
	}
	if !report.Complete {
		t.Error("complete data was marked incomplete")
	}
}

func TestBuildNoDataIsNotZeroEnergy(t *testing.T) {
	scheduled := time.Date(2024, 3, 2, 8, 0, 0, 0, time.UTC)
	in := ReportInput{Name: "report", Namespace: "ns", Period: "daily", Currency: "EUR", Price: "0.15", Scheduled: scheduled, Location: time.UTC, Devices: []Device{{Name: "missing", Interval: time.Minute}}}
	report, err := Build(context.Background(), fixedSource{missing: "missing"}, in)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(report.Chunks, "\n")
	if !strings.Contains(text, "Total (partial)</b>: N/A") || !strings.Contains(text, "Month to date: N/A") {
		t.Fatalf("missing data was not marked unavailable: %s", text)
	}
}

func TestSplitLinesCountsSupplementaryUnicode(t *testing.T) {
	chunks, err := splitLines([]string{"⚡😀😀", "⚡😀😀"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 2 {
		t.Fatalf("unexpected chunks: %v", chunks)
	}
}
