# Integrations

## Tapo

The collector uses the Tapo KLAP protocol and checks that the reported model starts with `P110`. The [device example](../examples/tapo-device.yaml) shows the address, polling interval, and account Secret references. The collector needs network access to the plug and rereads the mounted email and password before each poll. Failed polls back off up to five minutes. It only reads device information and energy measurements.

The simulator exercises the supported protocol path. Real-device validation is pending. To test a plug and its firmware, supply `TAPO_HOST`, `TAPO_EMAIL`, and `TAPO_PASSWORD` in your shell from your credential store, then run the opt-in read-only test from the source checkout.

```sh
KUBEWATTCH_HARDWARE_TEST=1 go test ./internal/collector -run '^TestTapoHardware$' -count=1
```

When sharing compatibility results, include the model and firmware and redact account and network details.

## Prometheus

### Scrape collector metrics

Each collector exposes `/metrics` over HTTPS on Service port `8443` (named `https`). The serving certificate must include the DNS SAN `<collector-service>.<device-namespace>.svc`. The Service name is `kw-` followed by the device UID with hyphens removed. Prometheus must trust the issuer and authenticate with a Kubernetes bearer token. Its ServiceAccount needs `get` on the non-resource URL `/metrics`.

With Prometheus Operator, install the ServiceMonitor CRD first, then enable `collector.serviceMonitor.enabled` and set the Prometheus ServiceAccount in Helm. The chart grants that account metrics-reader RBAC. The manager creates a ServiceMonitor in each device namespace with the configured discovery labels. Put the CA bundle in that namespace and set `spec.metrics.caSecretRef`. Prometheus must select both the monitor's labels and its namespace, and Prometheus Operator must be able to read the referenced CA. [Chart reference](../charts/kubewattch/README.md) · [Prometheus Operator troubleshooting](https://prometheus-operator.dev/docs/platform/troubleshooting/)

The generated monitors reference Prometheus's mounted ServiceAccount token using `bearerTokenFile`. Deployments that forbid ServiceMonitors from reading files in the Prometheus Pod reject these monitors. Keep the file restriction in place and configure the scrape from trusted Prometheus configuration when that policy applies. [Prometheus Operator API reference](https://prometheus-operator.dev/docs/api-reference/api/)

For a standalone or administrator-managed scrape, have a cluster administrator bind a role granting `get /metrics` to the Prometheus ServiceAccount. Mount the CA bundle in the Prometheus Pod and configure a scrape like this, substituting the collector Service name and namespace. Set a scrape interval at or below the device polling interval to reduce avoidable coverage gaps.

```yaml
scrape_configs:
  - job_name: kubewattch-collector
    scheme: https
    scrape_interval: 30s
    bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
    tls_config:
      ca_file: /etc/prometheus/kubewattch/ca.crt
      server_name: kw-DEVICEUID.energy-demo.svc
    static_configs:
      - targets: ['kw-DEVICEUID.energy-demo.svc:8443']
```

Manager metrics have a separate endpoint and trust configuration. Scrape the collector endpoint for energy measurements.

### Metric reference

Every series below has the labels `device_namespace`, `device`, and `provider` (currently `tapo`). Keep these labels when relabeling metrics. The reporter queries the power, collection success, and last success timestamp series.

| Metric | Type | Meaning |
| --- | --- | --- |
| `kubewattch_device_power_watts` | Gauge | Instantaneous power in watts. |
| `kubewattch_device_energy_today_joules` | Gauge | Energy so far in the device's current day, in joules. |
| `kubewattch_device_energy_month_joules` | Gauge | Energy so far in the device's current month, in joules. |
| `kubewattch_device_collection_success` | Gauge | `1` for a successful poll within twice the collection interval, otherwise `0`. |
| `kubewattch_device_last_success_timestamp_seconds` | Gauge | Unix timestamp of the last successful poll. Appears after the first success. |
| `kubewattch_device_collection_errors_total` | Counter | Failed polls since the collector process started. |

The two energy gauges reset on the device's own calendar. Divide joules by 3,600,000 for kWh. The collector omits power and energy samples when collection fails or becomes stale. The reporting calendar is configured independently on the report.

### Query Prometheus for reports

Set `spec.prometheus.url` to the HTTP(S) base URL reachable from the reporter Pod. Include a path prefix if Prometheus is served under one. The reporter appends `/api/v1/query_range`. Use HTTPS when sending a bearer token. Set `spec.prometheus.bearerTokenSecretRef` for token authentication and `spec.prometheus.caSecretRef` for a custom CA bundle. Both reference keys in the report namespace.

Retain data for the previous complete reporting period and the elapsed current month. A monthly report run late in the month can need nearly two months of history. Allow for both time and size retention limits.

## Telegram

Create a bot with BotFather. Start a conversation with the bot for a private chat, or add it to the intended group or channel and grant it permission to send messages. Obtain the chat ID, for example from an update returned by the Telegram Bot API after an interaction with the bot. Keep negative IDs as quoted strings in YAML. Treat the bot token as a credential when making API calls. [Telegram bot setup](https://core.telegram.org/bots/tutorial) · [Bot API getUpdates](https://core.telegram.org/bots/api#getupdates)

Store the bot token in a Secret in the report namespace and reference its name and key. Use a unique destination name for each chat. The reporter needs outbound HTTPS access to `api.telegram.org`. See the [daily report example](../examples/telegram-daily-report.yaml).
