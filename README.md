<div align="center">

# KubeWattch

### Kubernetes-native energy monitoring

<sub>Go · Kubebuilder · Prometheus</sub>

[![CI](https://github.com/alvarorg14/kubewattch/actions/workflows/ci.yml/badge.svg)](https://github.com/alvarorg14/kubewattch/actions/workflows/ci.yml)
[![Security](https://github.com/alvarorg14/kubewattch/actions/workflows/security.yml/badge.svg)](https://github.com/alvarorg14/kubewattch/actions/workflows/security.yml)
[![License](https://img.shields.io/github/license/alvarorg14/kubewattch)](LICENSE)

[Overview](#-overview) · [Quick Start](#-quick-start) · [Architecture](#-how-it-works) · [Contributing](CONTRIBUTING.md)

</div>

## 📋 Table of Contents

- [Overview](#-overview)
- [Why KubeWattch?](#-why-kubewattch)
- [Features and roadmap](#-features-and-roadmap)
- [Quick Start](#-quick-start)
- [Custom Resources](#-custom-resources)
- [Configuration](#️-configuration)
- [How It Works](#-how-it-works)
- [Local Development](#-local-development)
- [Contributing](CONTRIBUTING.md)
- [Security](#-security)
- [License](#-license)

## 📖 Overview

KubeWattch is an early-stage Kubernetes operator for monitoring smart-device energy consumption and reporting its cost. This repository currently contains API and controller scaffolding only.

## 🤔 Why KubeWattch?

The project aims to bring device configuration, Prometheus monitoring and scheduled consumption reports into a Kubernetes-native workflow. Its provider and destination integrations are planned for future iterations.

## ✨ Features and roadmap

- Namespaced EnergyDevice and EnergyReport APIs with generated CRDs.
- Operator manager and no-op controllers.
- Planned: Tapo polling, Prometheus energy metrics, scheduled Telegram reports, additional providers and destinations.

## 🚀 Quick Start

Install Go and run `make generate manifests` and `go build -o bin/manager ./cmd`. These commands do not connect to a cluster. The [getting started guide](docs/getting-started.md) describes chart installation with a locally built image. Integration examples describe planned behavior and do not collect energy data yet.

## 📡 Custom Resources

The API group is `energy.kubewattch.alvarorg14.github.io/v1alpha1`. See [CRDs](docs/crds.md) and [architecture](docs/architecture.md).

## ⚙️ Configuration

The chart configures the manager image, resources, leader election and optional authenticated Prometheus ServiceMonitor. See the [chart guide](charts/kubewattch/README.md). The API examples are illustrative and the controllers do not manage devices yet.

## 🔧 How It Works

```mermaid
flowchart LR
  CR[EnergyDevice and EnergyReport] --> Operator
  Operator -. future .-> Collector
  Collector -. future .-> Prometheus
  Prometheus -. future .-> Reporter
  Reporter -. future .-> Telegram
```

## 💻 Local Development

See [development](docs/development.md). Contributions follow Conventional Commits; see [CONTRIBUTING.md](CONTRIBUTING.md).

## 🔒 Security

Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).

## 📄 License

[Apache 2.0](LICENSE).
