package collector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	tapo "github.com/tess1o/tapo-go"
)

// Observation is a device measurement. Pointers distinguish missing values from zero.
type Observation struct {
	Time        time.Time
	PowerWatts  *float64
	TodayJoules *float64
	MonthJoules *float64
}

// Reader retrieves a single observation from a device.
type Reader interface {
	Read(context.Context) (Observation, error)
}

type safeRoundTripper struct{ base http.RoundTripper }

func (s safeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	response, err := s.base.RoundTrip(req)
	if err != nil {
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
	}
	return response, nil
}

type validatingTransport struct{ underlying tapo.Transport }

func (v validatingTransport) ExecuteRequest(ctx context.Context, req *tapo.RequestSpec) (json.RawMessage, error) {
	raw, err := v.underlying.ExecuteRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		ErrorCode *int            `json:"error_code"`
		Result    json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.ErrorCode == nil || *envelope.ErrorCode != 0 || len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return nil, errors.New("invalid device response")
	}
	if req.Method == "get_energy_usage" {
		var fields map[string]json.RawMessage
		if json.Unmarshal(envelope.Result, &fields) != nil {
			return nil, errors.New("invalid energy response")
		}
		for _, field := range []string{"current_power", "today_energy", "month_energy"} {
			var n int64
			value, ok := fields[field]
			if !ok || string(value) == "null" || json.Unmarshal(value, &n) != nil || n < 0 {
				return nil, errors.New("missing or invalid energy reading")
			}
		}
	}
	return raw, nil
}

// TapoReader owns a single P110 session and must be called sequentially.
type TapoReader struct {
	address, email, password string
	plug                     *tapo.SmartPlug
}

func NewTapoReader(address, email, password string) *TapoReader {
	return &TapoReader{address: address, email: email, password: password}
}

// Read bounds network requests by the caller's context and discards failed sessions.
func (r *TapoReader) Read(ctx context.Context) (obs Observation, err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("malformed device response")
		}
		if err != nil {
			r.plug = nil
		}
	}()
	if r.plug == nil {
		opts := tapo.Options{HttpClient: &http.Client{Timeout: 10 * time.Second, Transport: safeRoundTripper{base: http.DefaultTransport}}, RetryConfig: &tapo.RetryConfig{RetryCount: 0}}
		transport, e := tapo.NewKlapTransport(ctx, r.email, r.password, r.address, opts)
		if e != nil {
			return obs, errors.New("device connection failed")
		}
		r.plug = &tapo.SmartPlug{Device: tapo.NewDevice(validatingTransport{transport}, opts)}
		info, e := r.plug.DeviceInfo(ctx)
		if e != nil || info == nil || info.ErrorCode != 0 {
			return obs, errors.New("device identification failed")
		}
		if !strings.HasPrefix(strings.ToUpper(info.Result.Model), "P110") {
			return obs, fmt.Errorf("unsupported Tapo model")
		}
	}
	usage, e := r.plug.GetEnergyUsage(ctx)
	if e != nil || usage == nil || usage.ErrorCode != 0 {
		return obs, errors.New("energy collection failed")
	}
	power := float64(usage.Result.CurrentPower) / 1000
	today := float64(usage.Result.TodayEnergy) * 3600
	month := float64(usage.Result.MonthEnergy) * 3600
	return Observation{Time: time.Now(), PowerWatts: &power, TodayJoules: &today, MonthJoules: &month}, nil
}
