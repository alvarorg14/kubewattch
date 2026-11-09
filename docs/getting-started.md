# Getting started

This guide installs KubeWattch, collects a Tapo device's measurements into Prometheus, and sends a scheduled Telegram report. The chart, images, and example download links become available when [v0.2.0](https://github.com/alvarorg14/kubewattch/releases/tag/v0.2.0) is published. For builds and simulator tests from `main`, see [Development](development.md).

## Prerequisites

- A Kubernetes cluster you administer with Linux amd64 or arm64 nodes, plus Helm 3, kubectl, and curl. Scheduled reporting requires Kubernetes 1.32 or newer for the scheduled timestamp on CronJob-created Jobs. [Kubernetes CronJobs](https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/)
- A reachable Tapo P110-series device with energy monitoring and the account email and password used to set it up. Live hardware validation is pending.
- A Prometheus instance reachable from reporter Jobs. This walkthrough uses Prometheus Operator with the ServiceMonitor CRD installed. For a standalone server or a deployment that restricts file references in ServiceMonitors, see [Prometheus integration](integrations.md#prometheus).
- A trusted certificate issuer and a Telegram bot with permission to send to the intended chat. Prepare your bot token and chat ID using the [Telegram setup](integrations.md#telegram).

The operator watches all namespaces and its chart installs cluster-scoped RBAC and CRDs. This example installs the operator in `kubewattch-system` and keeps the custom resources and their Secrets in `energy-demo`. Commands use your current kubeconfig context.

## 1. Install the operator

Create a file called `kubewattch-values.yaml` for collector scraping. Set the ServiceAccount to the one used by your Prometheus Pods and use labels that match the Prometheus resource's `serviceMonitorSelector`. Its `serviceMonitorNamespaceSelector` must include `energy-demo`.

```yaml
collector:
  serviceMonitor:
    enabled: true
    labels:
      release: prometheus-example
    prometheusServiceAccount:
      name: prometheus
      namespace: monitoring
```

```sh
helm upgrade --install kubewattch oci://ghcr.io/alvarorg14/charts/kubewattch \
  --version 0.2.0 --namespace kubewattch-system --create-namespace \
  --values kubewattch-values.yaml --wait
kubectl --namespace kubewattch-system rollout status deployment/kubewattch-operator
kubectl create namespace energy-demo
```

This enables the device monitors. The operator's separate controller-runtime monitor is described in the [chart reference](../charts/kubewattch/README.md).

## 2. Configure the device

Save the Tapo email and password as separate local files with restricted permissions and no trailing newlines. Substitute your file paths in the Secret command. Keep credentials and generated private keys outside the repository.

```sh
kubectl --namespace energy-demo create secret generic tapo-credentials \
  --from-file=email=/path/to/tapo-email \
  --from-file=password=/path/to/tapo-password
curl --fail --silent --show-error --location --output tapo-device.yaml \
  https://raw.githubusercontent.com/alvarorg14/kubewattch/v0.2.0/examples/tapo-device.yaml
```

Edit `spec.tapo.address` in `tapo-device.yaml` to use a hostname or IP reachable from the collector Pod. Apply the resource first so that Kubernetes assigns its UID and you can determine the certificate's DNS name.

```sh
kubectl --namespace energy-demo apply -f tapo-device.yaml
device_uid=$(kubectl --namespace energy-demo get energydevice example-tapo -o jsonpath='{.metadata.uid}')
collector="kw-$(printf '%s' "$device_uid" | tr -d '-')"
printf 'Certificate DNS SAN: %s.energy-demo.svc\n' "$collector"
```

Issue a serving certificate for the printed DNS name. Use the serving certificate chain, private key, and trusted CA bundle from your issuer in these commands. The collector Pod waits for its TLS Secret to exist.

```sh
kubectl --namespace energy-demo create secret tls collector-tls \
  --cert=/path/to/collector-chain.pem --key=/path/to/collector-key.pem
kubectl --namespace energy-demo create secret generic collector-ca \
  --from-file=ca.crt=/path/to/collector-ca.pem
kubectl --namespace energy-demo wait --for=condition=Provisioned energydevice/example-tapo --timeout=180s
kubectl --namespace energy-demo wait --for=condition=Collecting energydevice/example-tapo --timeout=180s
```

In Prometheus, verify the collector target is up and run this query. A value of `1` confirms a fresh successful poll. View instantaneous power with `kubewattch_device_power_watts` using the same labels.

```promql
kubewattch_device_collection_success{device_namespace="energy-demo",device="example-tapo"}
```

## 3. Configure a scheduled report

Save the bot token in a restricted local file. Download the example, then set `spec.prometheus.url` to the Prometheus base URL reachable from the Job, `spec.destinations[0].telegram.chatID` to your chat ID as a string, and `spec.pricing` to your currency and tariff. For a secured Prometheus API, set the optional bearer token and CA references as described in the [API reference](crds.md#energyreport-spec).

```sh
kubectl --namespace energy-demo create secret generic telegram-credentials \
  --from-file=bot-token=/path/to/telegram-bot-token
curl --fail --silent --show-error --location --output telegram-daily-report.yaml \
  https://raw.githubusercontent.com/alvarorg14/kubewattch/v0.2.0/examples/telegram-daily-report.yaml
```

The example schedules a daily report at 08:00 Europe/Madrid and selects all devices in `energy-demo`. Adjust `spec.schedule`, `spec.timeZone`, and `spec.deviceSelector` as needed. For a quick delivery check, you can use `'* * * * *'` and restore the intended schedule after the first message. Each eligible scheduled run can send a message. A new installation has little history, so an early delivery can show partial or unavailable values.

```sh
kubectl --namespace energy-demo apply -f telegram-daily-report.yaml
kubectl --namespace energy-demo wait --for=condition=Provisioned energyreport/example-daily --timeout=180s
kubectl --namespace energy-demo get cronjobs,jobs
kubectl --namespace energy-demo get energyreport example-daily -o yaml
```

After the scheduled run, look for `Delivered=True` and `status.lastRun.state: Succeeded`, then check the chat. `DataComplete` reports data coverage separately from delivery. The reporter expects a Job created by the CronJob controller with its scheduled timestamp. See [Troubleshooting](troubleshooting.md) if either stage fails.

## Upgrade or uninstall

Follow the [CRD upgrade procedure](releases.md#upgrade-a-deployment) before upgrading the chart. On uninstall, delete the reports and devices that you want to retire while the manager is still running. This lets Kubernetes remove their owned workloads and lets the manager remove each collector's cluster-scoped authentication binding. Review resources in all namespaces before uninstalling a cluster-wide operator.

```sh
kubectl --namespace energy-demo delete energyreport example-daily
kubectl --namespace energy-demo delete energydevice example-tapo --wait=true
helm uninstall kubewattch --namespace kubewattch-system
```

Remove the example Secrets and namespaces separately if they are dedicated to this setup. Helm leaves the CRDs in place. Deleting a CRD also deletes all of its custom resources across the cluster. [Helm CRD handling](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/)
