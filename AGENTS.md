# KubeWattch: instructions for coding agents

## Project and current state

KubeWattch is an open-source Kubernetes operator for monitoring electricity use. The repository is a Go monorepo built with Kubebuilder `go/v4` and controller-runtime. Its module is `github.com/alvarorg14/kubewattch`; its Kubernetes API group is `energy.kubewattch.alvarorg14.github.io/v1alpha1`.

The initial delivery is a scaffold: `EnergyDevice` and `EnergyReport` have schemas and generated CRDs, but their reconcilers intentionally return without managing anything. There is no device polling, energy metrics exporter, report scheduler or Telegram delivery. Keep documentation and examples honest about this status. Do not add apparent integrations, binaries or empty provider packages merely to match a proposed directory tree.

## Repository map

- `api/v1alpha1/`: versioned public API types and generated DeepCopy code.
- `cmd/main.go`: controller-runtime manager, health endpoints, leader election and authenticated metrics.
- `internal/controller/`: resource watches, no-op reconciliations and envtest suite.
- `internal/collector/`, `internal/reporter/`: documented future package boundaries, not working implementations.
- `config/`: Kubebuilder-generated CRDs and RBAC, Kustomize deployment, manager and sample resources.
- `charts/kubewattch/`: Helm chart; `crds/` mirrors the generated CRDs in `config/crd/bases/`.
- `build/operator/`, `Dockerfile`: local operator image builds and the GoReleaser runtime-only image definition.
- `examples/`: synthetic resource specifications, not runnable integrations.
- `test/e2e/run.sh`: disposable Kind smoke test with an isolated temporary kubeconfig.
- `docs/`: design, API, setup, development, integration and release guides.
- `.github/workflows/`, `.goreleaser.yaml`, `renovate.json`, `release-please-config.json`, `build/components.json`: automation and release configuration.

## API and architecture rules

- Resources are namespaced; the manager watches all namespaces. A report selects devices only in its own namespace, and Secret references resolve there too.
- Preserve Kubernetes API conventions: optional fields, validation markers, structural schemas, DeepCopy generation, status subresources, observed generation and `metav1.Condition` semantics. Do not populate status with invented success.
- Only Tapo and Telegram configuration are modeled today. Add supported provider and destination types when implementing them, with validation and tests; avoid arbitrary maps and speculative interfaces.
- API versions and product releases are independent. Do not rename `v1alpha1` as part of a release. Treat incompatible API changes as compatibility work and document migrations.
- Cron and timezone schema markers validate shape only. Do not claim semantic validation or scheduling exists until it is implemented and tested.
- Separate the operator's reconciliation from future device collection and reporting. Grant RBAC only for operations implemented by the manager. Never place credentials, tokens, private addresses or chat IDs from real environments into examples, logs, tests or docs.
- Keep metrics protected by Kubernetes authentication/authorization and HTTPS. The chart's ServiceMonitor requires a trusted CA, serving certificate and Prometheus ServiceAccount binding; never silently disable certificate verification.

## Code generation and development

Go 1.27.3 or newer is required. The Makefile pins controller-gen, Kustomize and other tools. Use `make generate manifests` after changing API types or RBAC markers: it regenerates DeepCopy, CRDs and controller RBAC, and copies CRDs into the chart. Do not hand-edit generated output. Preserve Kubebuilder markers and `PROJECT` scaffold metadata.

For normal changes run the relevant checks; for API, workflow or project-wide changes use:

```sh
make generate manifests
make test GO_TEST_FLAGS=-race
make lint
make build
python3 hack/test-renovate-managers.py
python3 hack/test-pr-policy.py   # requires Node.js
python3 hack/check-release.py
```

`make test` downloads envtest binaries and starts a local API server; it does not use a cluster. `make test-e2e` requires Docker, Kind, Helm and kubectl, creates a uniquely named disposable cluster with its own temporary kubeconfig, and deletes it afterward. Never run `make install`, `make deploy`, `kubectl`, Helm installation or other cluster-changing commands against a user's cluster without explicit authorization. Do not modify the adjacent homelab repository.

Run Helm lint/template checks when chart or CRD changes. Check GoReleaser changes with the pinned `goreleaser check` and a non-publishing snapshot. Validate Actions changes with actionlint, and Renovate changes with `renovate-config-validator --strict`. If a required tool or Docker is unavailable, report precisely what was not verified; do not claim a check passed.

## Contributions and releases

Use Go formatting, small focused packages, clear error handling and meaningful tests. Update the relevant docs and examples when behavior changes. Contributors use Conventional Commit PR titles and exactly one policy label: `breaking-change`, `feature`, `enhancement`, `bug`, `dependencies`, `documentation`, `deprecations` or `ci`. Other labels are permitted. Labels are managed directly on GitHub, not synchronized by a workflow.

Release Please maintains one release version in `VERSION`, `.release-please-manifest.json` and chart metadata. GoReleaser builds and publishes the operator archives and image from the release tag; the inventory currently contains only the operator. Helm publishes the chart and CRD bundle separately. Preserve Release Please release notes and retry behavior. Maintain the GitHub App token design, pinned Actions and least-privilege workflow permissions. Renovate handles dependency updates; preserve the coordinated Kubernetes dependency group and tool version annotations.

Read [architecture](docs/architecture.md), [CRDs](docs/crds.md), [development](docs/development.md), [integrations](docs/integrations.md) and [releases](docs/releases.md) for details. Follow [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md) and the [code of conduct](CODE_OF_CONDUCT.md).
