# Integrations and homelab findings

The first planned use case is a Tapo P110 reporting instantaneous power and energy to Prometheus, followed by a daily Telegram summary with estimated cost. The current APIs model Tapo and Telegram configuration; the integrations themselves are not implemented. Shelly, MQTT, Discord and webhooks are possible future work.

## Current homelab patterns

A read-only review of the local homelab repository found:

- K3s is pinned to `v1.36.4+k3s1` and provisioned with Ansible. Argo CD manages application proxy charts using an app-of-apps pattern.
- The existing P110 setup uses the versioned `tess1o/go-tapo-exporter` image, a Deployment, Service and ServiceMonitor. Prometheus discovers the monitor using the `release: kube-prometheus-stack` label and scrapes every 30 seconds.
- kube-prometheus-stack provides Prometheus, Grafana and Alertmanager. Prometheus is configured for 30-day retention and an 8 GiB retention-size cap. Grafana discovers labeled dashboard ConfigMaps in `monitoring`.
- External Secrets obtains Tapo and Telegram credentials from 1Password and materializes Kubernetes Secrets. A CronJob sends a daily report at 08:00 Europe/Madrid. The setup also has a missing-metrics alert and a Tapo dashboard.

The existing report uses exporter-specific energy and power scaling and assumes 24-hour days. A replacement must account for provider units and resets, missing data, timezones and 23/25-hour daylight-saving days rather than relying on that assumption.

## Proposed adoption

Package the future operator chart as a pinned Helm proxy dependency, register it through the existing Argo CD app-of-apps and configure ServiceMonitor labels to match Prometheus discovery. Use namespace-local Secret references populated by the existing External Secrets workflow. Keep the present exporter and report running until measurements, reports and operational behavior have been compared and a migration is explicitly planned.

No homelab files, cluster resources, private addresses or credentials are included in this repository as a result of the review.

## Implementation sequence

1. Validate a Go Tapo client against the P110 protocol and normalize power to watts and energy to kilowatt-hours.
2. Implement polling, retry behavior, Secret rotation and documented Prometheus metric semantics.
3. Reconcile collector workloads and device health conditions.
4. Implement Prometheus-backed calendar-period calculations and fixed-price estimates.
5. Add Telegram delivery, scheduled report workloads, execution status and duplicate-send protection.
6. Add dashboards and a staged migration guide after end-to-end validation.
