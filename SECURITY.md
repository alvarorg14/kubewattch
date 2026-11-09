# Security policy

KubeWattch is an early-stage project. The project has no published security maintenance schedule or response-time commitment. Include the exact release or commit in a report so maintainers can assess the affected code.

## Reporting a vulnerability

Use the repository's [private GitHub reporting form](https://github.com/alvarorg14/kubewattch/security/advisories/new). Share the affected component and version, impact, reproduction steps, and any suggested mitigation. Keep vulnerability details private while maintainers investigate and coordinate disclosure. Use sanitized examples and omit actual credentials, tokens, kubeconfigs, private addresses, and chat IDs.

See the [Code of Conduct](CODE_OF_CONDUCT.md#enforcement) for conduct reports.

## Deployment and credentials

Collectors and reporters use mounted, namespace-local Secret references. Grant write access to the custom resources to trusted users who are allowed to choose those references and the endpoints the workloads access. The manager has cluster-wide reconciliation permissions.

Collector and manager metrics require Kubernetes authentication and authorization over HTTPS. Configure the scraping identity and a trusted CA for certificate verification. Use HTTPS for authenticated Prometheus API requests from reporters. [Architecture](docs/architecture.md#workloads-and-permissions) · [Prometheus integration](docs/integrations.md#prometheus)
