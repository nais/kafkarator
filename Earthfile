VERSION 0.8

FROM busybox

ARG --global REGISTRY=europe-north1-docker.pkg.dev/nais-io/nais/images/kafkarator

kubebuilder:
    FROM golang:1.24
    # Constants
    ARG os="linux"
    ARG arch="amd64"
    ARG kubebuilder_version="2.3.1"

    RUN wget -qO - https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${kubebuilder_version}/kubebuilder_${kubebuilder_version}_${os}_${arch}.tar.gz | tar -xz -C /tmp/
    SAVE ARTIFACT /tmp/kubebuilder_${kubebuilder_version}_${os}_${arch}/*
    SAVE IMAGE --cache-hint

dependencies:
    FROM golang:1.24
    # Go settings, needs to be ENV to be inherited into build
    ENV CGO_ENABLED=0
    ENV GOOS="linux"
    ENV GOARCH="amd64"
    ENV GO111MODULE=on

    COPY go.mod go.sum /workspace
    WORKDIR /workspace
    RUN go mod download
    SAVE IMAGE --cache-hint

build:
    FROM +dependencies
    COPY --dir +kubebuilder/ /usr/local/kubebuilder/
    COPY . /workspace
    RUN echo ${GOARCH} && make test
    RUN go build -installsuffix cgo -o kafkarator cmd/kafkarator/main.go
    RUN go build -installsuffix cgo -o canary cmd/canary/main.go

    SAVE ARTIFACT kafkarator
    SAVE ARTIFACT canary
    SAVE IMAGE --cache-hint

docker-kafkarator:
    FROM cgr.dev/chainguard/static:latest
    WORKDIR /
    COPY +build/kafkarator /
    CMD ["/kafkarator"]

    # builtins must be declared
    ARG EARTHLY_GIT_SHORT_HASH

    ARG kafkarator_image=${REGISTRY}/kafkarator
    ARG VERSION=$EARTHLY_GIT_SHORT_HASH
    SAVE IMAGE --push ${kafkarator_image}:${VERSION} ${kafkarator_image}:latest

docker-canary:
    FROM cgr.dev/chainguard/static:latest
    WORKDIR /
    COPY +build/canary /
    CMD ["/canary"]

    # builtins must be declared
    ARG EARTHLY_GIT_SHORT_HASH

    ARG canary_image=${REGISTRY}/canary
    ARG VERSION=$EARTHLY_GIT_SHORT_HASH
    SAVE IMAGE --push ${canary_image}:${VERSION} ${canary_image}:latest

uv-provider:
    FROM ghcr.io/astral-sh/uv:latest
    SAVE ARTIFACT /uv

docker-canary-deployer:
    FROM ghcr.io/nais/deploy/deploy:latest
    WORKDIR /canary/
    COPY +uv-provider/uv /usr/local/bin/
    COPY canary-deployer/requirements.txt /canary/
    RUN apk add python3 && \
        /usr/local/bin/uv pip install --system --requirement /canary/requirements.txt
    COPY canary-deployer/*.yaml /canary/
    COPY canary-deployer/deployer.py /canary/
    RUN python3 -c "import deployer" ## Minimal testing that imports actually work
    CMD ["python3", "/canary/deployer.py"]

    # builtins must be declared
    ARG EARTHLY_GIT_SHORT_HASH

    ARG canary_deployer_image=${REGISTRY}/canary-deployer
    ARG VERSION=$EARTHLY_GIT_SHORT_HASH
    SAVE IMAGE --push ${canary_deployer_image}:${VERSION} ${canary_deployer_image}:latest


docker:
    BUILD +docker-kafkarator
    BUILD +docker-canary
    BUILD +docker-canary-deployer
