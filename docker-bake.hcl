variable "VERSION" {
  default = "latest"
}

variable "REGISTRY" {
  default = "europe-north1-docker.pkg.dev/nais-io/nais/images/kafkarator"
}

group "default" {
  targets = ["kafkarator", "canary", "canary-deployer"]
}

target "kafkarator" {
  context    = "."
  dockerfile = "Dockerfile.kafkarator"
  platforms  = ["linux/amd64"]
  tags       = ["${REGISTRY}/kafkarator:${VERSION}", "${REGISTRY}/kafkarator:latest"]
}

target "canary" {
  context    = "."
  dockerfile = "Dockerfile.canary"
  platforms  = ["linux/amd64"]
  tags       = ["${REGISTRY}/canary:${VERSION}", "${REGISTRY}/canary:latest"]
}

target "canary-deployer" {
  context    = "."
  dockerfile = "Dockerfile.canary-deployer"
  platforms  = ["linux/amd64"]
  tags       = ["${REGISTRY}/canary-deployer:${VERSION}", "${REGISTRY}/canary-deployer:latest"]
}
