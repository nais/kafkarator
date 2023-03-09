# Kafkarator

Kafkarator is a Kubernetes operator on the [NAIS platform](https://doc.nais.io), providing
self-service functionality for Aiven hosted Kafka through Kubernetes resources.

Kafkarator defines a _Kubernetes custom resource_, `kafka.nais.io/Topic`. When users create or update this resource,
Kafkarator translates it to Aiven _topics_ and _ACL entries_.

![Kafkarator operator sequence diagram](doc/kafkarator.png)

## User documentation

* https://doc.nais.io/persistence/kafka/

## Developer documentation

Kafkarator uses [earthly](https://earthly.dev) via [earthlyw](https://github.com/mortenlj/earthlyw) for building.

Use `./earthlyw +docker` to build docker images for kafkarator and canary.

## Verifying the kafkarator images and their contents

The images are signed "keylessly" (is that a word?) using [Sigstore cosign](https://github.com/sigstore/cosign).
To verify their authenticity run
```
cosign verify \
--certificate-identity "https://github.com/nais/kafkarator/.github/workflows/main.yml@refs/heads/master" \
--certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
europe-north1-docker.pkg.dev/nais-io/nais/images/kafkarator@sha256:<shasum>
```

The images are also attested with SBOMs in the [CycloneDX](https://cyclonedx.org/) format.
You can verify these by running
```
cosign verify-attestation --type cyclonedx  \
--certificate-identity "https://github.com/nais/kafkarator/.github/workflows/main.yml@refs/heads/master" \
--certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
europe-north1-docker.pkg.dev/nais-io/nais/images/kafkarator@sha256:<shasum>
```

