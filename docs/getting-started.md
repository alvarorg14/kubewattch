# Getting started

This guide builds and installs the operator and collector. It configures a collector for a Tapo device; report delivery is not yet implemented.

## Build and test without a cluster

Install Go 1.27.3 or newer and Make, then run:

```sh
make generate manifests
make test
make build
```

`make test` downloads Kubernetes envtest binaries and starts a local API server and etcd. `make build` produces `bin/manager`. See [development](development.md) for the full validation suite.

## Install a locally built image

For an authorized Kubernetes cluster, install Docker, Helm and kubectl, and make sure your chosen image is available to the cluster:

```sh
make docker-build IMG=kubewattch/operator:dev
docker build -f build/collector/Dockerfile -t kubewattch/collector:dev .
# Load or publish the image using the method appropriate for your cluster.
helm install kubewattch ./charts/kubewattch \
  --namespace kubewattch-system --create-namespace \
  --set image.repository=kubewattch/operator --set image.tag=dev \
  --set collector.image.repository=kubewattch/collector --set collector.image.tag=dev
kubectl -n kubewattch-system rollout status deployment/kubewattch-operator
```

The chart installs the CRDs on first installation. An image tag named `dev` is illustrative; it must exist in the target cluster's image cache or registry. Release artifacts are not assumed to exist before the first release. Chart values, secure metrics configuration and CRD upgrade behavior are explained in the [chart guide](../charts/kubewattch/README.md).

For an isolated local test, `make test-e2e` creates and deletes its own Kind cluster. It requires Docker and network access.

## Explore the API

The [examples](../examples/) use documentation-only addresses and Secret references. Once the CRDs are installed, applying an example will create a Kubernetes object if its schema is valid. A device requires the referenced credentials and a metrics serving certificate to collect measurements. Report examples do not send messages. See [CRDs](crds.md) for fields, defaults and current validation limits.

## Device metrics

Create a TLS Secret in the device namespace with `tls.crt` and `tls.key`, then set `spec.metrics.servingCertSecretName`. The certificate must cover `<collector-service>.<namespace>.svc`; the collector Service name is `kw-` followed by the EnergyDevice UID without hyphens. A wildcard certificate for the namespace may also be used. Configure `spec.metrics.caSecretRef` and the chart's `collector.serviceMonitor` values to create a ServiceMonitor. Prometheus needs the corresponding trusted CA and ServiceAccount permissions; certificate verification is always enabled.

The metrics include power in watts and device-reported daily and monthly energy in joules. Divide energy by 3,600,000 for kWh. These energy readings reset according to the device's calendar and are gauges. Failed or stale polls omit energy and power readings and set collection success to zero.

For an opt-in read-only hardware test, provide `TAPO_HOST`, `TAPO_EMAIL` and `TAPO_PASSWORD` in the environment and run `KUBEWATTCH_HARDWARE_TEST=1 go test ./internal/collector -run TestTapoHardware`. Never commit these values. Hardware support must be validated against the actual firmware before relying on measurements.
