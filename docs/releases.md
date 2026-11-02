# Releases and dependency updates

## Versioning

The repository has one release version for the operator and Helm chart. `VERSION`, the Release Please manifest and the chart's `version`/`appVersion` are kept in sync. The chart defaults to the image tag `v<appVersion>`. The Kubernetes API remains `v1alpha1` until API evolution warrants a separate version; it is not tied to product releases.

The initial development version is `0.0.0`; the first configured release is `0.1.0`. Release Please examines Conventional Commits, maintains the changelog and opens a release PR. Its `ci` policy label satisfies the PR policy; release lifecycle labels are additional labels and do not count toward the one-category requirement. Maintainers manage labels directly on GitHub.

## Repository setup

Install a GitHub App with access to this repository and contents, pull requests and issues read/write permissions. Configure repository variable `RELEASE_APP_ID` and secret `RELEASE_APP_PRIVATE_KEY`. Enable creation of pull requests by GitHub Actions and set appropriate branch-protection checks. The GitHub App token allows release PRs to trigger CI. No credentials belong in the repository.

Install the hosted Renovate GitHub App for this repository. The checked-in configuration schedules routine updates on Monday mornings in Europe/Madrid, groups related updates, keeps major upgrades separate and disables automatic merging. Review Kubernetes and controller-runtime updates for compatibility. An update to a Kubebuilder tool version does not itself re-scaffold the project or change `PROJECT` provenance.

## Release flow

1. Merge reviewed PRs using Conventional Commit titles, preferably by squash merge.
2. `CI` runs on pull requests and pushes to `main`. After all CI jobs pass on a push to `main`, the Release Please job creates or updates a release PR. It skips stale CI runs when `main` has advanced, and does not run on PR or manual CI events. The separate Security workflow is not a dependency of this job.
3. Review the version, chart metadata and changelog; merging the release PR starts CI again. After CI succeeds, Release Please creates the GitHub release and tag.
4. The GitHub App token allows the published-release event to start the `Release` workflow. It publishes artifacts from the exact release tag. GoReleaser builds Linux amd64 and arm64 binaries, archives and checksums, pushes the multi-architecture operator image to GHCR, and uploads binary artifacts to the existing release without replacing its notes. Helm separately packages and pushes the OCI chart and attaches the CRD bundle and its checksums. Automatic publication handles stable releases; prereleases are skipped.
5. If publication fails after the release exists, run the `Release` workflow manually from `main` with that existing stable release tag. Ensure the workflow token has package write access and configure GHCR package visibility as appropriate.

`build/components.json` records the operator image and local-build Dockerfile; `.goreleaser.yaml` defines the actual release builds. Only the operator exists today. Add collector or reporter entries and corresponding GoReleaser builds only when their binaries and Dockerfiles exist. The GoReleaser Dockerfile copies prebuilt platform-specific binaries; the standard Dockerfiles remain available for local builds. `python3 hack/check-release.py` checks the current version and inventory.

Validate the pinned GoReleaser configuration with `OPERATOR_IMAGE=ghcr.io/alvarorg14/kubewattch/operator goreleaser check`. A non-publishing binary and archive snapshot can be built with `OPERATOR_IMAGE=ghcr.io/alvarorg14/kubewattch/operator goreleaser release --snapshot --clean --skip=docker`. This skips the image build and works without Docker. CI runs a full non-publishing snapshot with Docker Buildx and QEMU, which checks the per-platform images as well. GoReleaser replaces existing matching binary artifacts on a retry while retaining Release Please release notes.

## CRD upgrades

The chart installs generated CRDs from `crds/` on first installation. Helm does not upgrade or delete these CRDs during subsequent releases or uninstallation. Review the versioned CRD bundle and compatibility guidance, back up custom resources, and apply an approved CRD upgrade separately before upgrading the chart. Deleting a CRD deletes its custom resources; do not use that as an upgrade procedure.
