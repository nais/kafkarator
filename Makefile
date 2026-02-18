.PHONY: all build test fmt check generate

all: build test fmt check generate

build:
	mise run build

test:
	mise run test

fmt:
	mise run fmt

check:
	mise run check

generate:
	mise run generate
