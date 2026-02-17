# Kafkarator

Kafkarator is a Kubernetes operator on the [NAIS platform](https://doc.nais.io), providing
self-service functionality for Aiven-hosted Kafka through Kubernetes resources.

Kafkarator defines a _Kubernetes custom resource_, `kafka.nais.io/Topic`. When users create or update this resource,
Kafkarator translates it to Aiven _topics_ and _ACL entries_.

![Kafkarator operator sequence diagram](doc/kafkarator.png)

---

## Table of Contents
- [User documentation](#user-documentation)
- [Features](#features)
- [Architecture & Design](#architecture--design)
- [Configuration](#configuration)
- [Usage Example](#usage-example)
- [Scripts & Utilities](#scripts--utilities)
- [Developer documentation](#developer-documentation)
- [Helm Charts](#helm-charts)
- [Verifying Images](#verifying-the-kafkarator-images-and-their-contents)
- [Contributing](#contributing)
- [Links](#links)

---

## User documentation

- [NAIS Kafka docs](https://doc.nais.io/persistence/kafka/)

## Features

- Declarative management of Kafka topics and ACLs via Kubernetes CRDs.
- Automatic synchronization between Kubernetes resources and Aiven Kafka.
- Support for both topic and stream resources.
- Canary deployment and monitoring via the `canary` component.
- Helm charts for easy deployment and configuration.
- Security checks, static analysis, and SBOM attestation in CI.

## Architecture & Design

- Built as a Kubernetes operator using Go and controller-runtime.
- Uses a custom resource definition (CRD) `kafka.nais.io/Topic` for declarative Kafka management.
- [Architecture Decision Records (ADRs)](doc/adr/README.md) are maintained for key design decisions.
- **Note:** Future ADRs are maintained in the [PIG repository](https://github.com/navikt/pig).

## Configuration

- Most configuration is handled via environment variables, often with the `CANARY_` or `FEATURE_` prefix.
- Example variables:
  - `CANARY_KAFKA_BROKERS`: Comma-separated list of Kafka broker addresses.
  - `CANARY_KAFKA_CERTIFICATE_PATH`, `CANARY_KAFKA_KEY_PATH`, `CANARY_KAFKA_CA_PATH`: Paths to Kafka TLS credentials.
  - `CANARY_KAFKA_TOPIC`: Default topic for canary messages.
  - `CANARY_METRICS_ADDRESS`: Address for Prometheus metrics endpoint.
  - `FEATURE_GENERATED_CLIENT`: Feature flag for enabling generated client code.

See the `cmd/canary/main.go` and `cmd/kafkarator/feature_flags.go` for all available flags and environment variables.

## Usage Example

Deploy Kafkarator to your Kubernetes cluster using the provided Helm charts. Then create a `KafkaTopic` resource:

```yaml
apiVersion: kafka.nais.io/v1
kind: Topic
metadata:
  name: my-topic
  namespace: my-team
spec:
  pool: nav-dev
  config:
    partitions: 3
    replication: 2
  acl:
    - access: read
      application: my-app
      team: my-team
```

Kafkarator will automatically create the topic and set up ACLs in Aiven.

For more examples, see the [`examples/`](examples/) directory.

## Scripts & Utilities

The `scripts/` directory contains Python utilities for investigating and cleaning up Kafka users and ACLs. Key scripts include:
- `investigate_app.py`: Investigate service users and ACLs for a given app/team.
- `user_cleaner.py`: Find and optionally remove unused Kafka users.
- `find_app.py`: Search for secrets and users related to a specific app.

## Developer documentation

### Prerequisites
- [mise](https://mise.jdx.dev/) (for tool and task management)
- [Earthly](https://earthly.dev) (for reproducible builds)
- [Go](https://go.dev/) (see `mise.toml` for version)
- [Helm](https://helm.sh/) (for chart linting)

### Building and Testing

- **Build Docker images:**
  ```sh
  ./earthlyw +docker
  ```
  This uses the `earthlyw` wrapper to ensure you always use the correct Earthly version for reproducible builds. It builds Docker images for both `kafkarator` and `canary`.

- **Run all code checks (lint, static analysis, security, Helm lint):**
  ```sh
  mise run check
  ```
  This runs all checks defined in `.mise-tasks/check/` (deadcode, gosec, staticcheck, vulncheck, helm-lint, etc).

- **Run tests:**
  ```sh
  mise run test
  ```
  Or with coverage (if you have tests):
  ```sh
  mise run test -- --coverage
  ```

- **Build locally (binaries):**
  ```sh
  make kafkarator
  make canary
  ```
  This builds the binaries directly using Go.

### Helm Charts

Helm charts for `kafkarator`, `kafka-canary`, and `kafka-canary-alert` are in the `charts/` directory. Lint them with:
```sh
mise run check:helm-lint
```

To install a chart (example):
```sh
helm install kafkarator charts/kafkarator
```

### Verifying the kafkarator images and their contents

The images are signed "keylessly" using [Sigstore cosign](https://github.com/sigstore/cosign).
To verify their authenticity run (replace `<shasum>` with the actual image digest):
```sh
cosign verify \
  --certificate-identity "https://github.com/nais/kafkarator/.github/workflows/main.yml@refs/heads/master" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  europe-north1-docker.pkg.dev/nais-io/nais/images/kafkarator@sha256:<shasum>
```

The images are also attested with SBOMs in the [CycloneDX](https://cyclonedx.org/) format.
You can verify these by running:
```sh
cosign verify-attestation --type cyclonedx  \
  --certificate-identity "https://github.com/nais/kafkarator/.github/workflows/main.yml@refs/heads/master" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  europe-north1-docker.pkg.dev/nais-io/nais/images/kafkarator@sha256:<shasum>
```

### Contributing

- Please run `mise run check` and `mise run test` before submitting a PR.
- See `.mise-tasks/` for all available tasks.
- For questions, see [NAIS Slack](https://slack.com/app_redirect?channel=nais) or open an issue.
- Open issues or pull requests for new features or bug reports.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Links

- [NAIS Platform](https://doc.nais.io)
- [Aiven Kafka](https://aiven.io/kafka)
- [Earthly](https://earthly.dev)
- [mise](https://mise.jdx.dev/)
- [Helm](https://helm.sh/)
- [Architecture Decision Records](doc/adr/README.md)
