# Custom resource API

KubeWattch defines two namespaced resources in `energy.kubewattch.alvarorg14.github.io/v1alpha1`. Their schemas and status subresources are generated from the types in `api/v1alpha1`. Examples are available in [`examples/`](../examples/). The device controller manages collectors. The report controller schedules Prometheus-backed Telegram reports.

## EnergyDevice

| Field | Meaning |
| --- | --- |
| `spec.provider` | Required; currently only `tapo`. |
| `spec.tapo.address` | Required nonempty device hostname or IP address. |
| `spec.tapo.emailSecretRef` | Required Secret name and key for the Tapo account email. |
| `spec.tapo.passwordSecretRef` | Required Secret name and key for the account password. |
| `spec.collectionIntervalSeconds` | Positive interval, default `30`. |
| `spec.metrics.servingCertSecretName` | Same-namespace Secret containing `tls.crt` and `tls.key`. |
| `spec.metrics.caSecretRef` | Same-namespace CA Secret name and key, required when collector ServiceMonitor creation is enabled. |

Each Secret reference has `name` and `key`; both are required and the Secret is in the device namespace. The collector mounts the referenced Secrets and reads the selected keys.

## EnergyReport

| Field | Meaning |
| --- | --- |
| `spec.prometheus.url` | Prometheus HTTP API base URL. Required for execution. |
| `spec.prometheus.bearerTokenSecretRef` / `caSecretRef` | Optional same-namespace Secret keys for authentication and trusted CA. |
| `spec.suspend` | Suspend scheduled executions; defaults to false. |
| `spec.schedule` | Required five-field cron expression. |
| `spec.timeZone` | IANA timezone, default `UTC`. |
| `spec.deviceSelector` | Required Kubernetes label selector. An explicit `{}` selects all devices in the report namespace. |
| `spec.period` | `daily`, `weekly` or `monthly`. |
| `spec.pricing.currency` | Three uppercase letters, such as `EUR`. |
| `spec.pricing.pricePerKWh` | Nonnegative decimal string, such as `"0.15"`. |
| `spec.destinations` | Nonempty list of uniquely named destinations. |
| `spec.destinations[].type` | Currently only `telegram`. |
| `spec.destinations[].telegram.chatID` | Nonempty string identifying the destination chat. |
| `spec.destinations[].telegram.botTokenSecretRef` | Required name and key of a Secret in the report namespace. |

The CRD validates cron and timezone shape; the controller checks cron semantics and the IANA timezone before provisioning. Existing reports without `spec.prometheus` remain valid but cannot run until configured.

## Status and lifecycle

Both resources have an `observedGeneration` and Kubernetes conditions; device status also allows a last observation timestamp and report status allows last scheduled and successful times. The device controller reports provisioning and the collector reports collection health and successful observation time. The report controller and reporter update provisioning, delivery and data-completeness conditions, timestamps and last-run delivery checkpoints. A resource being accepted by the API server does not mean that a device has connected or a report will run.

The CRD API version is separate from release versions. Helm installs CRDs on initial installation but does not upgrade or delete chart CRDs automatically. Follow [release instructions](releases.md) before upgrading them.

## Report calculations

Reports cover the previous completed local day, Monday–Sunday week, or calendar month. Power is sampled from Prometheus at one-minute resolution and integrated over covered time to estimate kWh. Missing or stale samples reduce coverage and are not treated as zero. Estimated costs use the configured fixed tariff. Projections require at least one hour of the current month and 95% coverage for each device. Peaks are sampled values; aggregate peaks use simultaneous observations. Provision sufficient Prometheus retention for the chosen period and month-to-date data.

Delivery retries can produce a duplicate if Telegram accepted a message before confirmation was persisted. See [architecture](architecture.md) for lifecycle details.
