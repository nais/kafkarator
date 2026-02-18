#!/usr/bin/env sh
#MISE description="Run gosec"

set -e

go tool github.com/securego/gosec/v2/cmd/gosec --exclude-generated -terse ./...
