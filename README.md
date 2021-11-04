# Kafkarator

!!! Needs updating: This section is outdated

Kafkarator is a Kubernetes operator on the [NAIS platform](https://doc.nais.io), providing
self-service functionality for Aiven hosted Kafka through Kubernetes resources.

Kafkarator defines a _Kubernetes custom resource_, `kafka.nais.io/Topic`. When users create or update this resource,
Kafkarator _primary_ translates it to Aiven _topics_, _service users_ and _ACL entries_. Kafkarator then creates secrets
with user credentials, and produces them encrypted onto a special Kafka topic to facilitate multi-cluster operation.

Kafkarator _follower_ consumes the secrets and writes them into the Kubernetes cluster it's running on.

![Kafkarator operator sequence diagram](doc/kafkarator.png)

## User documentation

* https://doc.nais.io/addons/kafka

## Developer documentation

Kafkarator uses [earthly](https://earthly.dev) via [earthlyw](https://github.com/mortenlj/earthlyw) for building.

Use `./earthlyw +docker` to build docker images for kafkarator and canary.
