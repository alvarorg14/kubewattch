# KubeWattch Helm chart

The chart installs the operator manager, ServiceAccount, RBAC, metrics Service and the two CRDs. The operator provisions device collectors. The operator also provisions scheduled Telegram reporters; Tapo hardware validation is pending.

## Install

Build an operator image and make it available to the target cluster, then select it explicitly:

```sh
helm install kubewattch ./charts/kubewattch \
  --namespace kubewattch-system --create-namespace \
  --set image.repository=kubewattch/operator --set image.tag=dev
```

See [getting started](../../docs/getting-started.md) for prerequisites and isolated Kind testing. After releases are published, use the OCI chart and image for the desired release. The chart defaults to image tag `v<appVersion>` when `image.tag` is empty.

## Values

| Value | Default | Purpose |
| --- | --- | --- |
| `image.repository` | `ghcr.io/alvarorg14/kubewattch/operator` | Operator image repository. |
| `image.tag` | empty | Overrides the tag derived from `appVersion`. |
| `image.pullPolicy` | `IfNotPresent` | Container image pull policy. |
| `resources` | CPU and memory requests; memory limit | Manager resource configuration. |
| `reporter.image.repository` / `.tag` | reporter image / release tag | Reporter image configuration. |
| `reporter.resources` | CPU and memory requests; memory limit | Resources for each reporter Job. |
| `leaderElection.enabled` | `true` | Enable controller-runtime leader election. |
| `metrics.servingCertSecret` | empty | Optional Secret containing `tls.crt` and `tls.key` for the metrics server. |
| `metrics.serviceMonitor.enabled` | `false` | Render a Prometheus Operator ServiceMonitor and metrics-reader binding. |
| `metrics.serviceMonitor.labels` | `{}` | Additional discovery labels for the ServiceMonitor. |
| `metrics.serviceMonitor.caSecretName` / `caSecretKey` | empty / `ca.crt` | CA Secret and key in the ServiceMonitor namespace. |
| `metrics.serviceMonitor.prometheusServiceAccount.name` / `.namespace` | empty | Identity authorized to read metrics. |

The Deployment runs as nonroot with a read-only root filesystem, no privilege escalation and all capabilities dropped. The manager watches resources cluster-wide. The chart creates namespace-scoped Lease permissions and cluster-scoped permissions for resource watches and metrics authentication.

## Prometheus metrics

The manager's metrics endpoint is HTTPS with Kubernetes authentication and authorization. Without a supplied certificate the manager uses a self-signed certificate; that is useful for local exploration but does not provide a stable trust chain for Prometheus.

To enable the ServiceMonitor, provide a serving-certificate Secret in the release namespace with a DNS SAN for `<release>-operator-metrics.<namespace>.svc`. Supply the CA Secret name/key in the same namespace and the Prometheus ServiceAccount name/namespace. The chart binds that account to the metrics-reader ClusterRole. Add labels required by Prometheus discovery; the inspected homelab uses `release: kube-prometheus-stack`. Certificate verification remains enabled.

These are controller-runtime metrics. Collector metrics use separate Services and optional ServiceMonitors. Set `collector.serviceMonitor.enabled`, discovery labels and the Prometheus ServiceAccount, and configure the device TLS and CA Secret references.

## CRDs and upgrades

The chart's CRDs are copies of `config/crd/bases/`, synchronized by `make manifests`. Helm installs them when absent but does not upgrade or delete them. Review and apply a versioned CRD bundle separately before chart upgrades; back up custom resources and follow [release guidance](../../docs/releases.md). Removing a CRD deletes its custom resources.
