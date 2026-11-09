package reporter

import (
	"context"
	"errors"
	"fmt"
	"html"
	"math"
	"math/big"
	"sort"
	"strings"
	"time"
	"unicode/utf16"
)

// PowerSource provides observations for a device on the requested time grid.
type PowerSource interface {
	Power(context.Context, string, string, time.Duration, time.Time, time.Time) ([]Sample, error)
}

// Device identifies a namespace-local EnergyDevice selected at execution time.
type Device struct {
	Name     string
	Created  time.Time
	Interval time.Duration
}

// ReportInput is the immutable configuration used for a single scheduled run.
type ReportInput struct {
	Name, Namespace, Period, Currency, Price string
	Scheduled                                time.Time
	Location                                 *time.Location
	Devices                                  []Device
}

// Report is a rendered delivery snapshot and completeness indicator.
type Report struct {
	Chunks   []string
	Complete bool
}

// Build produces a deterministic HTML report. Calculations only use covered
// sample bins; the completeness flag does not imply billing-grade accuracy.
func Build(ctx context.Context, source PowerSource, in ReportInput) (Report, error) {
	start, end, err := Window(in.Scheduled, in.Location, in.Period)
	if err != nil {
		return Report{}, err
	}
	tariff, ok := new(big.Rat).SetString(in.Price)
	if !ok || tariff.Sign() < 0 || len(in.Currency) != 3 {
		return Report{}, errors.New("invalid tariff")
	}
	devices := append([]Device(nil), in.Devices...)
	sort.Slice(devices, func(i, j int) bool { return devices[i].Name < devices[j].Name })
	local := in.Scheduled.In(in.Location)
	monthStart := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, in.Location)
	nextMonth := monthStart.AddDate(0, 1, 0)
	yearStart := time.Date(local.Year(), 1, 1, 0, 0, 0, 0, in.Location)
	yearEnd := yearStart.AddDate(1, 0, 0)
	lines := []string{fmt.Sprintf("⚡ <b>%s</b>", html.EscapeString(in.Name)), fmt.Sprintf("Period: %s to %s (%s)", start.In(in.Location).Format("2006-01-02 15:04"), end.In(in.Location).Format("2006-01-02 15:04"), html.EscapeString(in.Location.String()))}
	complete := len(devices) > 0
	monthComplete := len(devices) > 0
	observedDevices := 0
	monthObserved := 0
	totalEnergy, monthEnergy := 0.0, 0.0
	current := 0.0
	currentCount := 0
	concurrent := map[int64]struct {
		count int
		watts float64
	}{}
	projectable := len(devices) > 0 && in.Scheduled.Sub(monthStart) >= time.Hour
	// Sum each device's observed average power for projections. This avoids
	// weighting a device with missing observations as though it were offline.
	projectedMean := 0.0
	for _, d := range devices {
		if d.Name == "" || d.Interval <= 0 {
			return Report{}, errors.New("invalid device")
		}
		samples, err := source.Power(ctx, in.Namespace, d.Name, d.Interval, start, end)
		if err != nil {
			return Report{}, err
		}
		filtered := samples[:0]
		for _, sample := range samples {
			if !sample.Time.Before(d.Created) {
				filtered = append(filtered, sample)
			}
		}
		m, err := Integrate(start, end, time.Minute, filtered)
		if err != nil {
			return Report{}, err
		}
		totalEnergy += m.EnergyKWh
		if m.Samples > 0 {
			observedDevices++
		}
		if m.Coverage() < 1 {
			complete = false
		}
		for _, sample := range filtered {
			if math.IsNaN(sample.Watts) || math.IsInf(sample.Watts, 0) || sample.Watts < 0 {
				continue
			}
			key := sample.Time.Unix()
			v := concurrent[key]
			v.count++
			v.watts += sample.Watts
			concurrent[key] = v
		}
		info := fmt.Sprintf("• <b>%s</b>: ", html.EscapeString(d.Name))
		if m.Samples == 0 {
			info += "N/A (coverage 0%)"
		} else {
			average, _ := m.AverageWatts()
			qualifier := ""
			if m.Coverage() < 1 {
				qualifier = " partial"
			}
			info += fmt.Sprintf("%.3f kWh%s; average %.1f W; sampled peak %.1f W; coverage %.1f%%; estimated cost %s %s", m.EnergyKWh, qualifier, average, m.PeakWatts, m.Coverage()*100, html.EscapeString(in.Currency), cost(m.EnergyKWh, tariff))
		}
		lines = append(lines, info)
		if in.Scheduled.After(monthStart) {
			samples, err = source.Power(ctx, in.Namespace, d.Name, d.Interval, monthStart, in.Scheduled)
			if err != nil {
				return Report{}, err
			}
			filtered = samples[:0]
			for _, sample := range samples {
				if !sample.Time.Before(d.Created) {
					filtered = append(filtered, sample)
				}
			}
			mm, err := Integrate(monthStart, in.Scheduled, time.Minute, filtered)
			if err != nil {
				return Report{}, err
			}
			monthEnergy += mm.EnergyKWh
			if mm.Samples > 0 {
				monthObserved++
			}
			if mm.Coverage() < 1 {
				monthComplete = false
			}
			if mm.Coverage() < 0.95 {
				projectable = false
			}
			avg, ok := mm.AverageWatts()
			if !ok {
				projectable = false
			} else {
				projectedMean += avg
			}
		} else {
			projectable = false
			monthComplete = false
		}
		recent := in.Scheduled.Add(-time.Minute)
		samples, err = source.Power(ctx, in.Namespace, d.Name, d.Interval, recent, in.Scheduled)
		if err != nil {
			return Report{}, err
		}
		if len(samples) > 0 && !samples[0].Time.Before(d.Created) {
			current += samples[0].Watts
			currentCount++
		}
	}
	if len(devices) == 0 {
		lines = append(lines, "No devices matched the selector.")
	} else {
		partial := ""
		if !complete {
			partial = " (partial)"
		}
		peak, peakOK := 0.0, false
		sumPower := 0.0
		commonCount := 0
		for _, v := range concurrent {
			if v.count == len(devices) {
				sumPower += v.watts
				commonCount++
				if !peakOK || v.watts > peak {
					peak = v.watts
					peakOK = true
				}
			}
		}
		peakText := "N/A"
		if peakOK {
			peakText = fmt.Sprintf("%.1f W", peak)
		}
		currentText := "N/A"
		if currentCount == len(devices) {
			currentText = fmt.Sprintf("%.1f W", current)
		}
		averageText := "N/A"
		if commonCount > 0 {
			averageText = fmt.Sprintf("%.1f W", sumPower/float64(commonCount))
		}
		totalText := "N/A"
		if observedDevices > 0 {
			totalText = fmt.Sprintf("%.3f kWh; estimated cost %s %s", totalEnergy, html.EscapeString(in.Currency), cost(totalEnergy, tariff))
		}
		lines = append(lines, fmt.Sprintf("\n<b>Total%s</b>: %s; simultaneous average %s; sampled peak %s", partial, totalText, averageText, peakText))
		monthPartial := ""
		if !monthComplete {
			monthPartial = " (partial)"
		}
		monthText := "N/A"
		if monthObserved > 0 {
			monthText = fmt.Sprintf("%.3f kWh%s", monthEnergy, monthPartial)
		}
		lines = append(lines, fmt.Sprintf("Month to date: %s; current sampled load: %s", monthText, currentText))
		if projectable {
			projectedMonth := projectedMean * nextMonth.Sub(monthStart).Seconds() / 3600000
			projectedYear := projectedMean * yearEnd.Sub(yearStart).Seconds() / 3600000
			lines = append(lines, fmt.Sprintf("Projected month: %.3f kWh (%s %s); annualized cost: %s %s", projectedMonth, html.EscapeString(in.Currency), cost(projectedMonth, tariff), html.EscapeString(in.Currency), cost(projectedYear, tariff)))
		} else {
			lines = append(lines, "Projections: N/A (insufficient coverage or elapsed time)")
		}
	}
	chunks, err := splitLines(lines, 4096)
	if err != nil {
		return Report{}, err
	}
	return Report{Chunks: chunks, Complete: complete && monthComplete && currentCount == len(devices)}, nil
}

func cost(kwh float64, tariff *big.Rat) string {
	r := new(big.Rat).SetFloat64(kwh)
	if r == nil {
		return "N/A"
	}
	return new(big.Rat).Mul(r, tariff).FloatString(2)
}

// Split on complete lines, ensuring HTML tags and entities are never cut.
func splitLines(lines []string, limit int) ([]string, error) {
	var result []string
	var chunk strings.Builder
	for _, line := range lines {
		if len(utf16.Encode([]rune(html.UnescapeString(line)))) > limit {
			return nil, errors.New("report line exceeds message limit")
		}
		candidate := line
		if chunk.Len() > 0 {
			candidate = "\n" + line
		}
		if len(utf16.Encode([]rune(html.UnescapeString(chunk.String()+candidate)))) > limit {
			result = append(result, chunk.String())
			chunk.Reset()
			candidate = line
		}
		chunk.WriteString(candidate)
	}
	if chunk.Len() > 0 {
		result = append(result, chunk.String())
	}
	return result, nil
}
