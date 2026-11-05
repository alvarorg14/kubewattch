package reporter

import (
	"errors"
	"math"
	"sort"
	"time"
)

// Window returns the last complete calendar period before the scheduled instant.
func Window(scheduled time.Time, location *time.Location, period string) (time.Time, time.Time, error) {
	if location == nil {
		return time.Time{}, time.Time{}, errors.New("missing timezone")
	}
	local := scheduled.In(location)
	end := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	switch period {
	case "daily":
		return end.AddDate(0, 0, -1), end, nil
	case "weekly":
		end = end.AddDate(0, 0, -(int(end.Weekday())+6)%7)
		return end.AddDate(0, 0, -7), end, nil
	case "monthly":
		end = time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, location)
		return end.AddDate(0, -1, 0), end, nil
	default:
		return time.Time{}, time.Time{}, errors.New("invalid report period")
	}
}

// Sample is a validated instantaneous power measurement at a sampling boundary.
type Sample struct {
	Time  time.Time
	Watts float64
}

// Measurement represents observed energy and coverage, rather than a prediction
// for the missing part of a window.
type Measurement struct {
	EnergyKWh float64
	Covered   time.Duration
	Duration  time.Duration
	PeakWatts float64
	Samples   int
}

func (m Measurement) Coverage() float64 {
	if m.Duration <= 0 {
		return 0
	}
	return float64(m.Covered) / float64(m.Duration)
}
func (m Measurement) AverageWatts() (float64, bool) {
	if m.Covered <= 0 {
		return 0, false
	}
	return m.EnergyKWh * 3600000 / m.Covered.Seconds(), true
}

// Integrate sums independent fixed-width sample bins, clipping at both window
// boundaries. Missing and invalid samples contribute no coverage. Duplicate
// timestamps are ignored, so repeated scrape series cannot double-count energy.
func Integrate(start, end time.Time, step time.Duration, samples []Sample) (Measurement, error) {
	result := Measurement{Duration: end.Sub(start)}
	if !end.After(start) || step <= 0 {
		return result, errors.New("invalid measurement window")
	}
	ordered := append([]Sample(nil), samples...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Time.Before(ordered[j].Time) })
	var previous time.Time
	for _, sample := range ordered {
		if math.IsNaN(sample.Watts) || math.IsInf(sample.Watts, 0) || sample.Watts < 0 {
			continue
		}
		if sample.Time.Before(start) || !sample.Time.Before(end) {
			continue
		}
		if sample.Time.Sub(start)%step != 0 {
			return Measurement{}, errors.New("sample is not aligned to window")
		}
		if !previous.IsZero() && sample.Time.Equal(previous) {
			continue
		}
		previous = sample.Time
		binEnd := sample.Time.Add(step)
		if binEnd.After(end) {
			binEnd = end
		}
		duration := binEnd.Sub(sample.Time)
		result.Covered += duration
		result.EnergyKWh += sample.Watts * duration.Seconds() / 3600000
		if result.Samples == 0 || sample.Watts > result.PeakWatts {
			result.PeakWatts = sample.Watts
		}
		result.Samples++
	}
	return result, nil
}
