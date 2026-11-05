package reporter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Prometheus queries one device's healthy, fresh power observations.
type Prometheus struct {
	URL         string
	BearerToken string
	Client      *http.Client
}

// Power queries a fixed 60-second grid, splitting long ranges into daily requests.
// The returned timestamps are evaluation timestamps, not device timestamps.
func (p Prometheus) Power(ctx context.Context, namespace, device string, interval time.Duration, start, end time.Time) ([]Sample, error) {
	base, err := url.Parse(p.URL)
	if err != nil || (base.Scheme != "https" && base.Scheme != "http") || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, errors.New("invalid Prometheus URL")
	}
	if namespace == "" || device == "" || interval <= 0 || !end.After(start) {
		return nil, errors.New("invalid Prometheus query parameters")
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	safeClient := *client
	safeClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	matcher := fmt.Sprintf("{device_namespace=%s,device=%s}", strconv.Quote(namespace), strconv.Quote(device))
	fresh := int64((2 * interval).Seconds())
	expression := fmt.Sprintf("max(kubewattch_device_power_watts%[1]s and (kubewattch_device_collection_success%[1]s == 1) and ((time() - kubewattch_device_last_success_timestamp_seconds%[1]s) <= %[2]d))", matcher, fresh)
	var samples []Sample
	for chunkStart := start; chunkStart.Before(end); chunkStart = chunkStart.Add(24 * time.Hour) {
		chunkEnd := chunkStart.Add(24 * time.Hour)
		if chunkEnd.After(end) {
			chunkEnd = end
		}
		last := chunkEnd.Add(-time.Millisecond)
		endpoint := *base
		endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/api/v1/query_range"
		values := url.Values{"query": {expression}, "start": {strconv.FormatFloat(float64(chunkStart.UnixNano())/1e9, 'f', -1, 64)}, "end": {strconv.FormatFloat(float64(last.UnixNano())/1e9, 'f', -1, 64)}, "step": {"60"}}
		endpoint.RawQuery = values.Encode()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
		if err != nil {
			return nil, errors.New("Prometheus request failed")
		}
		if p.BearerToken != "" {
			req.Header.Set("Authorization", "Bearer "+p.BearerToken)
		}
		resp, err := safeClient.Do(req)
		if err != nil {
			return nil, errors.New("Prometheus request failed")
		}
		var result struct {
			Status string `json:"status"`
			Data   struct {
				ResultType string `json:"resultType"`
				Result     []struct {
					Values [][]json.RawMessage `json:"values"`
				} `json:"result"`
			} `json:"data"`
		}
		decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&result)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK || decodeErr != nil || result.Status != "success" || result.Data.ResultType != "matrix" || len(result.Data.Result) > 1 {
			return nil, errors.New("Prometheus query failed or returned an invalid response")
		}
		for _, series := range result.Data.Result {
			for _, point := range series.Values {
				if len(point) != 2 {
					return nil, errors.New("invalid Prometheus sample")
				}
				var timestamp float64
				var value string
				if json.Unmarshal(point[0], &timestamp) != nil || json.Unmarshal(point[1], &value) != nil {
					return nil, errors.New("invalid Prometheus sample")
				}
				watts, err := strconv.ParseFloat(value, 64)
				if err != nil || math.IsNaN(watts) || math.IsInf(watts, 0) || watts < 0 || math.IsNaN(timestamp) || math.IsInf(timestamp, 0) {
					continue
				}
				seconds, fraction := math.Modf(timestamp)
				at := time.Unix(int64(seconds), int64(fraction*1e9))
				if at.Before(chunkStart) || !at.Before(chunkEnd) || at.Sub(start)%time.Minute != 0 {
					return nil, errors.New("invalid Prometheus sample timestamp")
				}
				samples = append(samples, Sample{Time: at, Watts: watts})
			}
		}
	}
	return samples, nil
}
