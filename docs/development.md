# Development

## Prerequisites

Use Go 1.27.3 or newer, Git, Make and Python 3. Node.js is needed for the PR-policy fixture test and Renovate validation. Helm and actionlint are needed for chart and workflow checks. Docker, Kind, Helm, kubectl, curl and OpenSSL are needed only for the disposable end-to-end test. Network access is required to download Go tools and envtest assets.

The Makefile pins controller-gen, Kustomize, golangci-lint and other CI tools. Its `tool-versions` target prints the versions used by workflows. The Go module version in `go.mod` controls the compiler version used by CI.

## Everyday checks

```sh
make generate manifests       # regenerate DeepCopy, CRDs, RBAC and chart CRD copies
make test GO_TEST_FLAGS=-race # envtest and Go tests
make lint                    # golangci-lint
make build                   # write bin/manager
```

Run generation after modifying API types or RBAC markers and include the generated changes in the same contribution. Do not edit DeepCopy files or generated CRDs manually. The controller envtest suite starts a local Kubernetes API server and etcd; it does not use the active kubeconfig.

For changes to automation or release metadata:

```sh
python3 hack/test-renovate-managers.py
python3 hack/test-pr-policy.py  # requires Node.js
python3 hack/test-release-gate.py
python3 hack/check-release.py
actionlint .github/workflows/*.yml
helm lint charts/kubewattch
```

Run Renovate's `renovate-config-validator --strict` when its configuration changes. Validate release changes with the pinned GoReleaser `check` and the non-publishing snapshot command in [releases](releases.md). CI also renders Kustomize and Helm resources, validates Kubernetes schemas and checks generation drift.

## End-to-end testing

`make test-e2e` builds the manager image and creates a uniquely named Kind cluster using a temporary kubeconfig. It installs the chart, runs a synthetic Tapo device, checks authenticated collector scraping with Prometheus, and exercises report delivery and duplicate-run recovery against a simulated Telegram endpoint. The script deletes the cluster on exit. It needs Docker and network access; never substitute a real cluster for this test.

`make install`, `make deploy` and their inverse targets operate on the current kubeconfig and must only be used with explicit authorization for the target cluster. See [getting started](getting-started.md) for a manual installation and [contributing](../CONTRIBUTING.md) for PR conventions.
