# Getting started

This guide builds and installs the current operator scaffold. It is useful for exploring the Kubernetes APIs and contributing; it does not collect energy consumption or send reports.

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
# Load or publish the image using the method appropriate for your cluster.
helm install kubewattch ./charts/kubewattch \
  --namespace kubewattch-system --create-namespace \
  --set image.repository=kubewattch/operator --set image.tag=dev
kubectl -n kubewattch-system rollout status deployment/kubewattch-operator
```

The chart installs the CRDs on first installation. An image tag named `dev` is illustrative; it must exist in the target cluster's image cache or registry. Release artifacts are not assumed to exist before the first release. Chart values, secure metrics configuration and CRD upgrade behavior are explained in the [chart guide](../charts/kubewattch/README.md).

For an isolated local test, `make test-e2e` creates and deletes its own Kind cluster. It requires Docker and network access.

## Explore the API

The [examples](../examples/) use documentation-only addresses and Secret references. Once the CRDs are installed, applying an example will create a Kubernetes object if its schema is valid. It will not connect to a Tapo plug or send Telegram messages. See [CRDs](crds.md) for fields, defaults and current validation limits.
