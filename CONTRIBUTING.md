# Contributing to KubeWattch

Use [GitHub Issues](https://github.com/alvarorg14/kubewattch/issues/new/choose) to report bugs or propose changes. For bugs, include the KubeWattch and Kubernetes versions, steps to reproduce, expected and actual behavior, and relevant status or logs. Redact credentials, private addresses, kubeconfigs, and chat IDs. Discuss substantial features or API changes in an issue before starting implementation.

See the [Security Policy](SECURITY.md) for private vulnerability reports and the [Code of Conduct](CODE_OF_CONDUCT.md) for participation and conduct reporting.

## Develop and validate

Fork the repository and create a topic branch from `main`. Use Go 1.27.3 or newer and the tools in the [development guide](docs/development.md). Keep the change focused and update the tests, examples, and docs that describe the behavior.

```sh
make generate manifests
make test GO_TEST_FLAGS=-race
make lint
make build
```

For API or RBAC changes, include the regenerated files. For chart or workflow changes, run the matching lint and render checks. The disposable Kind suite checks collector scraping and scheduled delivery. Use the [development guide](docs/development.md) for prerequisites and commands.

## Open a pull request

Use a [Conventional Commit](https://www.conventionalcommits.org/) title such as `feat: add device discovery`, `fix: reject an invalid secret reference`, or `docs: explain collector TLS setup`. Link the relevant issue and describe the change, how you validated it, and any upgrade implications. The title becomes the commit message when squash merged, so Release Please can use it to generate release notes.

Each PR must have exactly one policy label from `breaking-change`, `feature`, `enhancement`, `bug`, `dependencies`, `documentation`, `deprecations`, or `ci`. For documentation-only changes use `documentation`. If you lack label permissions, indicate the category in the PR so a maintainer can apply it. Other labels, including release lifecycle labels, may also be present.

See [Releases](docs/releases.md) for versioning, publication, and dependency update conventions.
