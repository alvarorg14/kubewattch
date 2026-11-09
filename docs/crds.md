# Custom resource API

`EnergyDevice` and `EnergyReport` are namespaced resources in `energy.kubewattch.alvarorg14.github.io/v1alpha1`. This reference describes their desired state and status. Kubernetes serves the generated field descriptions through `kubectl explain energydevice.spec` and `kubectl explain energyreport.spec`.

Each Secret key reference has two required fields, `name` and `key`. The Secret must be in the namespace of the resource that references it. Supply the exact keys referenced by the spec. See [Getting started](getting-started.md) for example Secrets.

## EnergyDevice spec

| Field | Requirement or default | Purpose |
| --- | --- | --- |
| `spec.provider` | Required, `tapo` | Device provider. |
| `spec.tapo` | Required for `tapo` | Tapo connection settings. |
| `spec.tapo.address` | Required, nonempty | Device hostname or IP address reachable from the collector Pod. |
| `spec.tapo.emailSecretRef` | Required | Tapo account email key. |
| `spec.tapo.passwordSecretRef` | Required | Tapo account password key. |
| `spec.collectionIntervalSeconds` | Defaults to `30`, minimum `1` | Wait in seconds after a successful poll. Failures use backoff. |
| `spec.metrics` | Optional in the schema, required to run the collector | Collector metrics settings. |
| `spec.metrics.servingCertSecretName` | Required when `metrics` is present | Serving TLS Secret containing `tls.crt` and `tls.key`. |
| `spec.metrics.caSecretRef` | Required for generated collector ServiceMonitors | Key containing the CA bundle Prometheus uses to verify the serving certificate. |

See the [Tapo device example](../examples/tapo-device.yaml) and [certificate setup](integrations.md#scrape-collector-metrics).

## EnergyReport spec

| Field | Requirement or default | Purpose |
| --- | --- | --- |
| `spec.prometheus` | Optional in the schema, required to schedule the report | Prometheus query settings. |
| `spec.prometheus.url` | Required when `prometheus` is present | HTTP(S) base URL including the host and optional path prefix. The controller rejects URL credentials, query strings, and fragments. |
| `spec.prometheus.bearerTokenSecretRef` | Optional | Key containing the Prometheus API bearer token. Use an HTTPS URL. |
| `spec.prometheus.caSecretRef` | Optional | Key containing an additional PEM CA bundle for Prometheus HTTPS verification. |
| `spec.suspend` | Defaults to `false` | Suspend future CronJob schedules. |
| `spec.schedule` | Required, five-field cron | When the report runs in the configured timezone. |
| `spec.timeZone` | Defaults to `UTC` | IANA timezone for the schedule and report windows, such as `Europe/Madrid`. |
| `spec.deviceSelector` | Required | Kubernetes label selector supporting `matchLabels` and `matchExpressions`. Use `{}` for all devices in the report namespace. |
| `spec.period` | Required, `daily`, `weekly`, or `monthly` | Previous complete local day, Monday–Sunday week, or calendar month. Set the cron schedule separately. |
| `spec.pricing` | Required | Fixed electricity tariff. |
| `spec.pricing.currency` | Required, three uppercase letters | Currency shown in estimated costs, such as `EUR`. |
| `spec.pricing.pricePerKWh` | Required, nonnegative decimal string | Fixed tariff per kWh, such as `"0.15"`. |
| `spec.destinations` | Required, at least one entry | List of Telegram destinations. |
| `spec.destinations[].name` | Required, nonempty and unique in the report | Identifier used for delivery checkpoints. |
| `spec.destinations[].type` | Required, `telegram` | Delivery integration. |
| `spec.destinations[].telegram` | Required for `telegram` | Bot and chat settings. |
| `spec.destinations[].telegram.chatID` | Required, nonempty string | Destination chat ID. Quote numeric and negative IDs in YAML. |
| `spec.destinations[].telegram.botTokenSecretRef` | Required | Key containing the Telegram bot token. |

The API server checks cron and timezone shape. The controller also parses the schedule and timezone and validates the Prometheus URL and selector before provisioning. An existing report without Prometheus settings is accepted by the schema and shows `Provisioned=False` until configured. See the [Telegram report example](../examples/telegram-daily-report.yaml).

## Status

Both resources use standard Kubernetes conditions. Compare a condition's `observedGeneration` with `metadata.generation` before interpreting it for the current spec. Top-level `status.observedGeneration` records the generation the manager observed.

| Resource | Condition | Meaning |
| --- | --- | --- |
| `EnergyDevice` | `Provisioned` | The collector Deployment is ready and the manager has reconciled its configuration. |
| `EnergyDevice` | `Collecting` | The collector's latest poll succeeded. Use the freshness metrics to detect a stalled collector. |
| `EnergyReport` | `Provisioned` | The CronJob and supporting resources have been configured. |
| `EnergyReport` | `DataComplete` | The latest evaluated report had complete data for its reporting window, month to date, and latest sampled load. |
| `EnergyReport` | `Delivered` | The latest execution reached all configured destinations when true. |

`EnergyDevice.status.lastObservationTime` records the most recent successful poll. For a report, `status.lastScheduleTime` tracks scheduling and `status.lastSuccessfulTime` tracks successful delivery. `status.lastRun` records the scheduled time, resource generation, execution state (`Running`, `Succeeded`, or `Failed`), and confirmed chunk counts by destination.

See [Architecture](architecture.md#report-windows-and-calculations) for report calculations and [Troubleshooting](troubleshooting.md) for common status reasons. The CRD API version is independent of the chart and binary release versions. [CRD upgrades](releases.md#upgrade-a-deployment)
