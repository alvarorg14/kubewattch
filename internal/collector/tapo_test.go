package collector

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	tapo "github.com/tess1o/tapo-go"
)

type fakeTransport struct {
	data string
	err  error
}

func (f fakeTransport) ExecuteRequest(context.Context, *tapo.RequestSpec) (json.RawMessage, error) {
	return json.RawMessage(f.data), f.err
}

func TestValidateEnergyResponse(t *testing.T) {
	cases := []struct {
		name, data string
		valid      bool
	}{
		{"zero", `{"error_code":0,"result":{"current_power":0,"today_energy":0,"month_energy":0}}`, true},
		{"missing", `{"error_code":0,"result":{"today_energy":0,"month_energy":0}}`, false},
		{"null", `{"error_code":0,"result":{"current_power":null,"today_energy":0,"month_energy":0}}`, false},
		{"negative", `{"error_code":0,"result":{"current_power":-1,"today_energy":0,"month_energy":0}}`, false},
		{"device error", `{"error_code":-1,"result":{}}`, false},
		{"malformed", `{`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := (validatingTransport{fakeTransport{data: tc.data}}).ExecuteRequest(context.Background(), &tapo.RequestSpec{Method: "get_energy_usage"})
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
		})
	}
	_, err := (validatingTransport{fakeTransport{err: errors.New("network")}}).ExecuteRequest(context.Background(), &tapo.RequestSpec{Method: "get_energy_usage"})
	if err == nil {
		t.Fatal("expected transport error")
	}
}

func TestTapoReaderNormalizesEnergy(t *testing.T) {
	transport := validatingTransport{fakeTransport{data: `{"error_code":0,"result":{"current_power":12500,"today_energy":1000,"month_energy":2500}}`}}
	reader := &TapoReader{plug: &tapo.SmartPlug{Device: tapo.NewDevice(transport, tapo.Options{})}}
	obs, err := reader.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if *obs.PowerWatts != 12.5 || *obs.TodayJoules != 3600000 || *obs.MonthJoules != 9000000 {
		t.Fatalf("unexpected normalized reading: %+v", obs)
	}
}
