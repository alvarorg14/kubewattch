# Development

Use the `main` branch for development, source builds, and simulator tests.

```sh
git clone https://github.com/alvarorg14/kubewattch.git
cd kubewattch
git switch main
```

## Prerequisites

Use Go 1.27.3 or newer, Git, Make, and Python 3. Node.js runs the PR-policy fixture test and the Renovate validator. Install Helm for chart checks and actionlint for workflow checks. Network access is needed to download Go modules, pinned tools, and envtest binaries. The Makefile's `tool-versions` target lists tool pins, and `go.mod` defines the Go version used by CI.

## Everyday checks

```sh
make generate manifests
make test GO_TEST_FLAGS=-race
make lint
make build
```

Run generation after editing API types or RBAC markers and commit the generated DeepCopy code, CRDs, RBAC, and chart CRD copies that change. The test suite starts a local Kubernetes API server and etcd through envtest. `make build` writes `bin/manager`, `bin/collector`, and `bin/reporter`.

For chart and automation changes, run the relevant checks.

```sh
helm lint charts/kubewattch
helm template test charts/kubewattch --include-crds
python3 hack/test-renovate-managers.py
python3 hack/test-pr-policy.py
python3 hack/check-release.py
python3 hack/test-release-gate.py
actionlint .github/workflows/*.yml
```

Render the secure monitor values as well when changing that configuration. The [CI workflow](../.github/workflows/ci.yml) shows the manager ServiceMonitor flags. Run `renovate-config-validator --strict` for Renovate changes. See [Releases](releases.md#validate-a-snapshot) for GoReleaser checks.

## Disposable Kind smoke test

Run `make test-e2e` with Docker, Kind, Helm, kubectl, curl, OpenSSL, and Python 3. The script builds the manager, collector, test reporter, and simulator images. It creates a unique Kind cluster with a temporary kubeconfig and deletes the cluster on exit. It checks that Prometheus can scrape and store simulated Tapo measurements over authenticated HTTPS. It also exercises scheduled delivery to a simulated Telegram endpoint and confirms that replaying the same completed run does not send another message.

```sh
make test-e2e
```

Use the separate [opt-in hardware test](integrations.md#tapo) to check a real device.

## Install images built from source

To try your local changes in a cluster you administer, build all three images.

```sh
make docker-build IMG=kubewattch/operator:dev
docker build -f build/collector/Dockerfile -t kubewattch/collector:dev .
docker build -f build/reporter/Dockerfile -t kubewattch/reporter:dev .
```

Load the images into your cluster nodes or push them to a reachable registry and adjust the repository overrides. For an existing Kind cluster you administer, run `kind load docker-image kubewattch/operator:dev kubewattch/collector:dev kubewattch/reporter:dev --name YOUR_CLUSTER`. Install the chart from the same checkout.

```sh
helm upgrade --install kubewattch ./charts/kubewattch \
  --namespace kubewattch-system --create-namespace --wait \
  --set image.repository=kubewattch/operator --set image.tag=dev \
  --set collector.image.repository=kubewattch/collector --set collector.image.tag=dev \
  --set reporter.image.repository=kubewattch/reporter --set reporter.image.tag=dev
```

Use the [getting started guide](getting-started.md#2-configure-the-device) with the files in the local [examples directory](../examples/). For collector ServiceMonitors, pass the values shown in the [install step](getting-started.md#1-install-the-operator). Commands such as `make install` and `make deploy` use your active kubeconfig. Confirm the target before running them.
