# Releases and dependency updates

## Versioning

A single product version covers the manager, collector, reporter, and Helm chart. Release Please keeps `VERSION`, the Release Please manifest, and the chart's `version` and `appVersion` in sync. When image tags are left empty, the chart uses `v<appVersion>`. The Kubernetes API version `v1alpha1` has its own lifecycle.

The [v0.2.0 release](https://github.com/alvarorg14/kubewattch/releases/tag/v0.2.0) is the release referenced by the user guides. Its install links and artifacts become available upon publication. Use `main` for [source builds](development.md#install-images-built-from-source) and [snapshot validation](#validate-a-snapshot).

## Upgrade a deployment

Helm installs missing CRDs on first installation. Apply the matching CRD bundle separately before upgrading the chart. Review release notes, back up existing custom resources from all namespaces, and use the intended cluster context. Store the backup where only authorized users can access it. [Helm CRD handling](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/)

The following example upgrades to `v0.2.0` once published.

```sh
kubectl get energydevices,energyreports --all-namespaces -o yaml > kubewattch-resources-backup.yaml
curl --fail --silent --show-error --location --output kubewattch-crds-0.2.0.yaml \
  https://github.com/alvarorg14/kubewattch/releases/download/v0.2.0/kubewattch-crds-0.2.0.yaml
kubectl apply -f kubewattch-crds-0.2.0.yaml
helm upgrade kubewattch oci://ghcr.io/alvarorg14/charts/kubewattch \
  --version 0.2.0 --namespace kubewattch-system --wait
```

Pass your values file to `helm upgrade` if you use one. Check the manager rollout and the resources' status after upgrading. Devices from the scaffold release need a serving certificate to provision collectors. Reports need a Prometheus URL and working Telegram credentials before they can run. See [Getting started](getting-started.md) for the configuration. Deleting a CRD deletes its custom resources across all namespaces.

## Maintainer setup

Configure a GitHub App with contents, pull requests, and issues read/write permissions on the repository. Set `RELEASE_APP_ID` as a repository variable and `RELEASE_APP_PRIVATE_KEY` as a Secret. The App token lets Release Please PRs trigger CI and published releases trigger the Release workflow. Configure the repository's Actions and branch protection settings for this workflow.

Install the hosted Renovate GitHub App. The checked-in configuration schedules routine updates on Monday mornings in Europe/Madrid, groups related updates, keeps major upgrades separate, and disables automatic merging. Review the Kubernetes and controller-runtime updates together. Tool updates leave `PROJECT` scaffold provenance tied to the original scaffold.

## Release flow

1. Merge reviewed PRs with Conventional Commit titles. Release Please uses the commit history and maintains the changelog and release PR. The release PR uses the `ci` policy label in addition to its lifecycle labels.
2. CI runs on pull requests and pushes to `main`. Release Please runs after successful CI on the current `main` push. The Security workflow runs independently.
3. Review the release PR's version, chart metadata, and changelog. Merge it to trigger CI and let Release Please create the GitHub release and tag.
4. A published stable release triggers the Release workflow. GoReleaser uses the tagged source to build Linux amd64 and arm64 archives and checksums and push the three multi-architecture images to GHCR. Helm pushes the OCI chart and uploads the chart archive, CRD bundle, and `SHA256SUMS` to GitHub. `build/components.json` lists the images and local-build Dockerfiles.
5. If artifact publication fails, run the Release workflow manually from `main` for the existing stable release tag. Check package write access and GHCR visibility. Matching artifacts can be replaced on retry and Release Please's notes are retained.

## Validate a snapshot

Use the GoReleaser version printed by `make -s tool-versions` and the image repositories from the component inventory.

```sh
export OPERATOR_IMAGE=ghcr.io/alvarorg14/kubewattch/operator
export COLLECTOR_IMAGE=ghcr.io/alvarorg14/kubewattch/collector
export REPORTER_IMAGE=ghcr.io/alvarorg14/kubewattch/reporter
goreleaser check
goreleaser release --snapshot --clean --skip=docker
python3 hack/check-release.py
```

The snapshot command builds local binaries and archives. CI runs a full snapshot with Docker Buildx and QEMU to check the per-platform images as well.
