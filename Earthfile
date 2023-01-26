VERSION 0.6

FROM busybox

kubebuilder:
    FROM curlimages/curl:latest
    # Constants
    ARG os="linux"
    ARG arch="amd64"
    ARG kubebuilder_version="2.3.1"

    RUN curl -L https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${kubebuilder_version}/kubebuilder_${kubebuilder_version}_${os}_${arch}.tar.gz | tar -xz -C /tmp/
    SAVE ARTIFACT /tmp/kubebuilder_${kubebuilder_version}_${os}_${arch}/*
    SAVE IMAGE --cache-hint

dependencies:
    FROM golang:1.19
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
    FROM alpine:3
    WORKDIR /
    COPY +build/kafkarator /
    CMD ["/kafkarator"]

    # builtins must be declared
    ARG EARTHLY_GIT_PROJECT_NAME
    ARG EARTHLY_GIT_SHORT_HASH

    ARG kafkarator_image=ghcr.io/$EARTHLY_GIT_PROJECT_NAME/kafkarator
    ARG VERSION=$EARTHLY_GIT_SHORT_HASH
    SAVE IMAGE --push ${kafkarator_image}:${VERSION} ${kafkarator_image}:latest

docker-canary:
    FROM alpine:3
    WORKDIR /
    COPY +build/canary /
    CMD ["/canary"]

    # builtins must be declared
    ARG EARTHLY_GIT_PROJECT_NAME
    ARG EARTHLY_GIT_SHORT_HASH

    ARG canary_image=ghcr.io/$EARTHLY_GIT_PROJECT_NAME/canary
    ARG VERSION=$EARTHLY_GIT_SHORT_HASH
    SAVE IMAGE --push ${canary_image}:${VERSION} ${canary_image}:latest

docker-canary-deployer:
    FROM ghcr.io/nais/deploy/deploy:latest
    RUN apk add python3 && \
        python3 -m ensurepip && \
        pip3 install pyaml trio pydantic
    COPY --dir canary-deployer /canary
    CMD ["python3", "/canary/deployer.py"]

    # builtins must be declared
    ARG EARTHLY_GIT_PROJECT_NAME
    ARG EARTHLY_GIT_SHORT_HASH

    ARG canary_deployer_image=ghcr.io/$EARTHLY_GIT_PROJECT_NAME/canary-deployer
    ARG VERSION=$EARTHLY_GIT_SHORT_HASH
    SAVE IMAGE --push ${canary_deployer_image}:${VERSION} ${canary_deployer_image}:latest


docker:
    BUILD +docker-kafkarator
    BUILD +docker-canary
    BUILD +docker-canary-deployer
