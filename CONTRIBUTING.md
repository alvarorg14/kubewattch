# Contributing to KubeWattch

Thanks for helping improve KubeWattch. Please follow the [code of conduct](CODE_OF_CONDUCT.md) and discuss substantial features or API changes in an issue before investing in a large implementation. Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).

## Development

Install Go 1.27.3 or newer and the tools listed in [development](docs/development.md). Create a topic branch from `main`, keep changes focused, and include tests and documentation appropriate to the change. Run the relevant checks before opening a PR:

```sh
make generate manifests
make test GO_TEST_FLAGS=-race
make lint
make build
```

API and RBAC changes require regenerated DeepCopy code, CRDs, RBAC and chart CRD copies. Chart and workflow changes also require their lint and rendering checks. The disposable Kind test is available with `make test-e2e` when Docker is available. Never use another person's cluster or homelab for tests without explicit authorization.

## Pull requests

Use a [Conventional Commit](https://www.conventionalcommits.org/) title, for example `feat: add device discovery` or `fix: reject an invalid secret reference`. Describe the problem, behavior, validation and any upgrade impact in the PR template. Prefer squash merge so the title becomes the commit message consumed by Release Please.

Apply exactly one policy label: `breaking-change`, `feature`, `enhancement`, `bug`, `dependencies`, `documentation`, `deprecations` or `ci`. Other labels, including Release Please lifecycle labels, are allowed. Maintainers manage labels directly in GitHub. Labels describe the change; commit messages determine release versions.

Renovate proposes dependency updates. Review Kubernetes modules and controller-runtime compatibility together, and keep pinned workflow Actions and development tools current. See [releases](docs/releases.md) for versioning and publication details.
