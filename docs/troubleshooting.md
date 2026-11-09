# Troubleshooting

These commands assume the release and resource names from [Getting started](getting-started.md). Substitute your namespace and names as needed. Redact credentials, private hostnames and addresses, and chat IDs before sharing logs or manifests.

## Operator or resources unavailable

```sh
helm status kubewattch --namespace kubewattch-system
kubectl get crd energydevices.energy.kubewattch.alvarorg14.github.io \
  energyreports.energy.kubewattch.alvarorg14.github.io
kubectl --namespace kubewattch-system get pods
kubectl --namespace kubewattch-system logs deployment/kubewattch-operator -c manager
```

If the CRD is missing or fields are rejected after an upgrade, check the installed schema with `kubectl explain energydevice.spec` or `kubectl explain energyreport.spec` and follow the [CRD upgrade procedure](releases.md#upgrade-a-deployment). For `ImagePullBackOff`, check Pod events for the exact repository and tag and verify registry access from the nodes. A local image must be loaded on the nodes or pushed to a registry they can reach.

## Collector not ready or not collecting

```sh
namespace=energy-demo
device=example-tapo
device_uid=$(kubectl --namespace "$namespace" get energydevice "$device" -o jsonpath='{.metadata.uid}')
collector="kw-$(printf '%s' "$device_uid" | tr -d '-')"
kubectl --namespace "$namespace" describe energydevice "$device"
kubectl --namespace "$namespace" describe deployment "$collector"
kubectl --namespace "$namespace" get pods --selector "energy.kubewattch.alvarorg14.github.io/device-uid=$device_uid"
kubectl --namespace "$namespace" get events --sort-by=.metadata.creationTimestamp
kubectl --namespace "$namespace" logs "deployment/$collector" -c collector
```

| Symptom | Check |
| --- | --- |
| `MissingMetricsCertificate` | Set `spec.metrics.servingCertSecretName`. If the collector Pod shows `FailedMount`, verify the named TLS Secret and its `tls.crt` and `tls.key` keys in the device namespace. |
| `MissingMetricsCA` | Set `spec.metrics.caSecretRef` when collector ServiceMonitors are enabled. Verify the referenced Secret and key. |
| `MissingCollectorImage` or `CollectorNotReady` | Check the manager's collector image configuration, then inspect Pod events for image, mount, and readiness errors. |
| `Provisioned=True` with `Collecting=False` | Check the Tapo Secret names and keys, account credentials, model and firmware, and network access from the Pod to the device. The collector retries with backoff. |
| Condition belongs to an older generation | Compare the condition's `observedGeneration` with `metadata.generation` and inspect the manager logs for reconciliation errors. |

If a device remains in deletion, check the manager and its permission to delete the collector's ClusterRoleBinding. The collector finalizer waits for that cleanup.

## Prometheus target missing, down, or stale

```sh
kubectl --namespace "$namespace" get servicemonitor "$collector" -o yaml
kubectl --namespace "$namespace" get service "$collector"
kubectl --namespace "$namespace" get endpointslice --selector "kubernetes.io/service-name=$collector"
```

- **Target missing.** Verify the ServiceMonitor CRD, monitor labels and namespace selectors, and that Prometheus Operator accepts the monitor. If it rejects the ServiceAccount token file reference, use the [administrator-managed scrape configuration](integrations.md#scrape-collector-metrics).
- **TLS error.** Check that the certificate covers `$collector.$namespace.svc`, is current, and chains to the CA referenced by the monitor. Recreating the device changes the Service name and can require a new certificate.
- **HTTP 401 or 403.** For 401, check that Prometheus sends a valid Kubernetes token. For 403, verify that the actual Prometheus ServiceAccount is bound to a role allowing `get /metrics`. A cluster administrator can check with `kubectl auth can-i get /metrics --as=system:serviceaccount:monitoring:prometheus` after substituting the correct account.
- **Target up with missing power.** Run the queries below. A success value of zero means the latest poll failed or the last successful poll is stale. If all series are absent, check scrape errors, whether you are scraping the collector rather than the manager, and whether relabeling preserved the device labels.

```promql
kubewattch_device_collection_success{device_namespace="energy-demo",device="example-tapo"}
```

```promql
time() - kubewattch_device_last_success_timestamp_seconds{device_namespace="energy-demo",device="example-tapo"}
```

## Report missing, failed, or incomplete

```sh
namespace=energy-demo
report=example-daily
report_uid=$(kubectl --namespace "$namespace" get energyreport "$report" -o jsonpath='{.metadata.uid}')
cronjob="kwr-$(printf '%s' "$report_uid" | tr -d '-')"
kubectl --namespace "$namespace" describe energyreport "$report"
kubectl --namespace "$namespace" describe cronjob "$cronjob"
kubectl --namespace "$namespace" get jobs --selector "energy.kubewattch.alvarorg14.github.io/report-uid=$report_uid"
```

Choose the relevant Job from the list and inspect its events and Pod logs.

```sh
job=REPLACE_WITH_JOB_NAME
kubectl --namespace "$namespace" describe job "$job"
kubectl --namespace "$namespace" logs "job/$job" -c reporter
```

| Symptom | Check |
| --- | --- |
| `Provisioned=False` | Read the condition reason and message. Check `spec.prometheus.url`, cron syntax, timezone, selector, and reporter resource quantities. |
| CronJob exists but no Job runs | Check `spec.suspend`, the next scheduled time in the configured timezone, and CronJob events for missed schedules or an active Job blocking overlap. |
| Job fails before querying | Check the image and the Job's owner and `batch.kubernetes.io/cronjob-scheduled-timestamp` annotation. Kubernetes 1.32 or newer adds it on a scheduled run. Wait for a scheduled run after fixing configuration. |
| Prometheus query fails | Check DNS, network policy, the base URL and optional prefix, and the referenced token and CA keys. Verify that the Job can reach `/api/v1/query_range`. |
| `Delivered=False` | Check the Telegram token Secret key, chat ID, bot permissions, and outbound HTTPS. Inspect `status.lastRun.destinations` for confirmed message chunks. |
| `Delivered=True` with `DataComplete=False` | Check whether the selector matches devices in the report namespace and whether Prometheus retained fresh samples for the selected period and month to date. A newly created device can have partial history. |

See [API status fields](crds.md#status) and [report calculations](architecture.md#report-windows-and-calculations).
