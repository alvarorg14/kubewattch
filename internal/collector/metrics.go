package collector

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds the most recent collection result and implements prometheus.Collector.
type Metrics struct {
	mu                                        sync.RWMutex
	observation                               Observation
	success                                   bool
	lastSuccess                               time.Time
	failures                                  uint64
	interval                                  time.Duration
	power, today, month, result, last, errors *prometheus.Desc
}

func NewMetrics(namespace, device, provider string, interval time.Duration) *Metrics {
	labels := prometheus.Labels{"device_namespace": namespace, "device": device, "provider": provider}
	desc := func(name, help string) *prometheus.Desc { return prometheus.NewDesc(name, help, nil, labels) }
	return &Metrics{interval: interval,
		power:  desc("kubewattch_device_power_watts", "Instantaneous device power in watts."),
		today:  desc("kubewattch_device_energy_today_joules", "Energy used during the device's current day in joules."),
		month:  desc("kubewattch_device_energy_month_joules", "Energy used during the device's current month in joules."),
		result: desc("kubewattch_device_collection_success", "Whether the most recent collection succeeded."),
		last:   desc("kubewattch_device_last_success_timestamp_seconds", "Timestamp of the last successful collection."),
		errors: desc("kubewattch_device_collection_errors_total", "Number of failed collection cycles."),
	}
}

func (m *Metrics) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{m.power, m.today, m.month, m.result, m.last, m.errors} {
		ch <- d
	}
}
func (m *Metrics) Collect(ch chan<- prometheus.Metric) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ok := m.success && time.Since(m.lastSuccess) <= 2*m.interval
	value := 0.0
	if ok {
		value = 1
	}
	ch <- prometheus.MustNewConstMetric(m.result, prometheus.GaugeValue, value)
	ch <- prometheus.MustNewConstMetric(m.errors, prometheus.CounterValue, float64(m.failures))
	if !m.lastSuccess.IsZero() {
		ch <- prometheus.MustNewConstMetric(m.last, prometheus.GaugeValue, float64(m.lastSuccess.Unix()))
	}
	if !ok {
		return
	}
	for _, pair := range []struct {
		desc  *prometheus.Desc
		value *float64
	}{{m.power, m.observation.PowerWatts}, {m.today, m.observation.TodayJoules}, {m.month, m.observation.MonthJoules}} {
		if pair.value != nil {
			ch <- prometheus.MustNewConstMetric(pair.desc, prometheus.GaugeValue, *pair.value)
		}
	}
}

// PollOnce updates the collection state atomically.
func (m *Metrics) PollOnce(ctx context.Context, reader Reader) error {
	obs, err := reader.Read(ctx)
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		m.success = false
		m.failures++
		return err
	}
	m.observation = obs
	m.lastSuccess = obs.Time
	m.success = true
	return nil
}
