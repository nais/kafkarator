FROM busybox

# builtins must be declared
ARG EARTHLY_GIT_PROJECT_NAME
ARG EARTHLY_GIT_SHORT_HASH

# Override from command-line on CI
ARG cache_image=ghcr.io/$EARTHLY_GIT_PROJECT_NAME/cache
ARG kafkarator_image=ghcr.io/$EARTHLY_GIT_PROJECT_NAME/kafkarator
ARG canary_image=ghcr.io/$EARTHLY_GIT_PROJECT_NAME/canary
ARG VERSION=$EARTHLY_GIT_SHORT_HASH

# Constants
ARG os="linux"
ARG arch="amd64"
ARG kubebuilder_version="2.3.1"

# Go settings
ARG CGO_ENABLED=0
ARG GOOS=${os}
ARG GOARCH=${arch}
ARG GO111MODULE=on

kubebuilder:
    FROM curlimages/curl:latest
    RUN echo ${os}
    RUN echo ${arch}
    RUN curl -L https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${kubebuilder_version}/kubebuilder_${kubebuilder_version}_${os}_${arch}.tar.gz | tar -xz -C /tmp/
    SAVE ARTIFACT /tmp/kubebuilder_${kubebuilder_version}_${os}_${arch}/*
    SAVE IMAGE --push ${cache_image}:kubebuilder

dependencies:
    FROM golang:1.17
    COPY go.mod go.sum /workspace
    WORKDIR /workspace
    RUN go mod download
    SAVE IMAGE --push ${cache_image}:dependencies

build:
    FROM +dependencies
    COPY --dir +kubebuilder/ /usr/local/kubebuilder/
    COPY . /workspace
    RUN make test
    RUN go build -installsuffix cgo -o kafkarator cmd/kafkarator/main.go
    RUN go build -installsuffix cgo -o canary cmd/canary/main.go
    SAVE ARTIFACT kafkarator
    SAVE ARTIFACT canary
    SAVE IMAGE --push ${cache_image}:build

docker-kafkarator:
    FROM alpine:3
    WORKDIR /
    COPY +build/kafkarator /
    CMD ["/kafkarator"]
    SAVE IMAGE --push ${kafkarator_image}:${VERSION}
    SAVE IMAGE --push ${kafkarator_image}:latest

docker-canary:
    FROM alpine:3
    WORKDIR /
    COPY +build/canary /
    CMD ["/canary"]
    SAVE IMAGE --push ${canary_image}:${VERSION}
    SAVE IMAGE --push ${canary_image}:latest

docker:
    BUILD +docker-kafkarator
    BUILD +docker-canary
