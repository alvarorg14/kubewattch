# KubeWattch: instructions for coding agents

## Project and current state

KubeWattch is an open-source Kubernetes operator for monitoring electricity use. The repository is a Go monorepo built with Kubebuilder `go/v4` and controller-runtime. Its module is `github.com/alvarorg14/kubewattch`; its Kubernetes API group is `energy.kubewattch.alvarorg14.github.io/v1alpha1`.

`EnergyDevice` manages a collector for each configured Tapo P110-series device. `EnergyReport` manages a CronJob that queries Prometheus and sends reports to Telegram. Simulator coverage exercises both features. Live Tapo hardware validation is pending. Keep the supported integrations and tested scope clear in docs and examples.

## Repository map

- `api/v1alpha1/`: versioned public API types and generated DeepCopy code.
- `cmd/main.go`: controller-runtime manager, health endpoints, leader election and authenticated metrics.
- `cmd/collector/`, `cmd/reporter/`: collector and reporter workload entry points.
- `internal/controller/`: workload reconciliation, resource watches and envtest suite.
- `internal/collector/`, `internal/reporter/`: Tapo polling and metrics, and Prometheus queries, report calculations and Telegram delivery.
- `config/`: Kubebuilder-generated CRDs and RBAC, Kustomize deployment, manager and sample resources.
- `charts/kubewattch/`: Helm chart; `crds/` mirrors the generated CRDs in `config/crd/bases/`.
- `build/`: local and GoReleaser image definitions and component inventory for the manager, collector and reporter.
- `examples/`: resource manifests with synthetic addresses, credential references and chat IDs.
- `test/e2e/run.sh`: disposable Kind smoke test with an isolated kubeconfig and simulated Tapo and Telegram endpoints.
- `docs/`: design, API, setup, development, integration and release guides.
- `.github/workflows/`, `.goreleaser.yaml`, `renovate.json`, `release-please-config.json`, `build/components.json`: automation and release configuration.

## API and architecture rules

- Resources are namespaced; the manager watches all namespaces. A report selects devices only in its own namespace, and Secret references resolve there too.
- Preserve Kubernetes API conventions: optional fields, validation markers, structural schemas, DeepCopy generation, status subresources, observed generation and `metav1.Condition` semantics. Do not populate status with invented success.
- Tapo is the supported provider and Telegram is the supported destination. Add new types with implementation, validation and tests. Avoid arbitrary maps and speculative interfaces.
- API versions and product releases are independent. Do not rename `v1alpha1` as part of a release. Treat incompatible API changes as compatibility work and document migrations.
- The schema validates cron and timezone shape. The report controller validates schedule and timezone semantics and provisions a CronJob. The reporter requires the scheduled timestamp annotation introduced in Kubernetes 1.32.
- Keep reconciliation, collection and reporting in separate workloads. Scope RBAC to each workload's operations. Never place credentials, tokens, private addresses or chat IDs from real environments into examples, logs, tests or docs.
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
python3 hack/test-release-gate.py
```

`make test` downloads envtest binaries and starts a local API server; it does not use a cluster. `make test-e2e` requires Docker, Kind, Helm and kubectl, creates a uniquely named disposable cluster with its own temporary kubeconfig, and deletes it afterward. Never run `make install`, `make deploy`, `kubectl`, Helm installation or other cluster-changing commands against a user's cluster without explicit authorization. Do not modify the adjacent homelab repository.

Run Helm lint/template checks when chart or CRD changes. Check GoReleaser changes with the pinned `goreleaser check` and a non-publishing snapshot. Validate Actions changes with actionlint, and Renovate changes with `renovate-config-validator --strict`. If a required tool or Docker is unavailable, report precisely what was not verified; do not claim a check passed.

## Contributions and releases

Use Go formatting, small focused packages, clear error handling and meaningful tests. Update the relevant docs and examples when behavior changes. Contributors use Conventional Commit PR titles and exactly one policy label: `breaking-change`, `feature`, `enhancement`, `bug`, `dependencies`, `documentation`, `deprecations` or `ci`. Other labels are permitted. Labels are managed directly on GitHub, not synchronized by a workflow.

CI runs on PRs and pushes to `main`. Only a successful, current `main` push runs Release Please. A published stable GitHub Release triggers artifact publication. Manual Release dispatch retries an existing tag. Security runs independently. Release Please maintains one release version in `VERSION`, `.release-please-manifest.json` and chart metadata. GoReleaser builds and publishes archives and images for the operator, collector and reporter from the release tag. Helm publishes the chart and CRD bundle separately. Preserve Release Please release notes and retry behavior, the GitHub App token design, pinned Actions and least-privilege workflow permissions. Renovate handles dependency updates. Preserve the coordinated Kubernetes dependency group and tool version annotations.

Read [architecture](docs/architecture.md), [CRDs](docs/crds.md), [development](docs/development.md), [integrations](docs/integrations.md) and [releases](docs/releases.md) for details. Follow [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md) and the [code of conduct](CODE_OF_CONDUCT.md).
