# Security policy

KubeWattch is an early-stage project and has no published support or security maintenance schedule yet. Do not use its current API objects as evidence that device monitoring or reporting is operational.

## Reporting a vulnerability

Please report suspected vulnerabilities privately through [GitHub private vulnerability reporting](https://github.com/alvarorg14/kubewattch/security/advisories/new). Include affected versions or commits, reproduction steps, impact and any suggested mitigation. Do not post secrets, private device addresses or exploit details in public issues. Maintainers will review the report and coordinate a fix and disclosure as appropriate.

## Credential handling

Examples use synthetic values and Kubernetes Secret references. Never commit actual device credentials, bot tokens, kubeconfigs or private infrastructure configuration. Collectors and reporters mount the referenced Secret keys; their identities do not need Secret API access. Integrations must grant the minimum permissions required and avoid logging sensitive data.
