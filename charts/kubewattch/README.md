# KubeWattch Helm chart

The chart installs the manager, its ServiceAccount and RBAC, its HTTPS metrics Service, and the two CRDs. The manager provisions collectors for devices and scheduled reporter Jobs for reports. The chart and images below become available upon publication of [v0.2.0](https://github.com/alvarorg14/kubewattch/releases/tag/v0.2.0).

```sh
helm upgrade --install kubewattch oci://ghcr.io/alvarorg14/charts/kubewattch \
  --version 0.2.0 --namespace kubewattch-system --create-namespace --wait
```

See [Getting started](../../docs/getting-started.md) for resource configuration and [Development](../../docs/development.md) for source images and local chart installs.

## Values

### Operator

| Value | Default | Purpose |
| --- | --- | --- |
| `image.repository` | `ghcr.io/alvarorg14/kubewattch/operator` | Manager image repository. |
| `image.tag` | `""` | Image tag override. Empty uses `v<appVersion>`. |
| `image.pullPolicy` | `IfNotPresent` | Manager image pull policy. |
| `resources.requests.cpu` | `10m` | Manager CPU request. |
| `resources.requests.memory` | `64Mi` | Manager memory request. |
| `resources.limits.memory` | `128Mi` | Manager memory limit. |
| `leaderElection.enabled` | `true` | Coordinate manager leadership using a Lease. |

The manager runs as non-root with a read-only root filesystem. Its `resources` map accepts Kubernetes container resource requests and limits and applies to the manager Deployment.

### Manager metrics

| Value | Default | Purpose |
| --- | --- | --- |
| `metrics.servingCertSecret` | `""` | Serving TLS Secret name in the release namespace. |
| `metrics.serviceMonitor.enabled` | `false` | Render a manager ServiceMonitor and metrics-reader RBAC. |
| `metrics.serviceMonitor.labels` | `{}` | Labels used by Prometheus to discover the monitor. |
| `metrics.serviceMonitor.caSecretName` | `""` | Secret with the trusted CA in the release namespace. |
| `metrics.serviceMonitor.caSecretKey` | `ca.crt` | Key containing the trusted CA bundle. |
| `metrics.serviceMonitor.prometheusServiceAccount.name` | `""` | ServiceAccount used by the scraping Prometheus Pods. |
| `metrics.serviceMonitor.prometheusServiceAccount.namespace` | `""` | Namespace of that ServiceAccount. |

Without a supplied certificate, the manager uses a self-signed certificate. When enabling its ServiceMonitor, install the ServiceMonitor CRD and supply the serving certificate, CA, and Prometheus ServiceAccount. The serving Secret must have `tls.crt` and `tls.key` and the certificate must cover `<release>-operator-metrics.<namespace>.svc`.

### Collector

| Value | Default | Purpose |
| --- | --- | --- |
| `collector.image.repository` | `ghcr.io/alvarorg14/kubewattch/collector` | Image repository for collector Pods. |
| `collector.image.tag` | `""` | Image tag override. Empty uses `v<appVersion>`. |
| `collector.serviceMonitor.enabled` | `false` | Have the manager create a ServiceMonitor for each device and grant the Prometheus ServiceAccount scrape access. |
| `collector.serviceMonitor.labels` | `{}` | Discovery labels for generated device monitors. |
| `collector.serviceMonitor.prometheusServiceAccount.name` | `""` | Required when enabled. ServiceAccount used by the scraping Prometheus Pods. |
| `collector.serviceMonitor.prometheusServiceAccount.namespace` | `""` | Required when enabled. Namespace of that ServiceAccount. |

Serving TLS and the CA reference are configured on each `EnergyDevice`. When enabling collector monitors, install the ServiceMonitor CRD and configure Prometheus discovery for the device namespaces. Both monitor types use authenticated HTTPS with certificate verification. [Prometheus integration](../../docs/integrations.md#prometheus)

### Reporter

| Value | Default | Purpose |
| --- | --- | --- |
| `reporter.image.repository` | `ghcr.io/alvarorg14/kubewattch/reporter` | Image repository for reporter Jobs. |
| `reporter.image.tag` | `""` | Image tag override. Empty uses `v<appVersion>`. |
| `reporter.resources.requests.cpu` | `10m` | CPU request for each reporter container. |
| `reporter.resources.requests.memory` | `64Mi` | Memory request for each reporter container. |
| `reporter.resources.limits.memory` | `256Mi` | Memory limit for each reporter container. |

Reporter resources must be positive Kubernetes quantities. Configure the schedule, tariff, Prometheus endpoint, and Telegram destinations per [EnergyReport](../../docs/crds.md#energyreport-spec).

## CRD lifecycle

The chart installs missing CRDs during initial installation. Apply the matching versioned CRD bundle separately before upgrading an existing deployment. The chart CRDs mirror the generated files in `config/crd/bases/`. See [Release and upgrade instructions](../../docs/releases.md#upgrade-a-deployment).
