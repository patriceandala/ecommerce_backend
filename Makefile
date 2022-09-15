SHELL := /usr/bin/env bash

# github access token to build private module
GITHUB_TOKEN ?= unset

REGISTRY 	:= asia-southeast1-docker.pkg.dev
PROJECT 	?= svc-prod-340502
REPOSITORY 	?= services

# install our required Go tools for development and to build the program.
.PHONY: install-tools
install-tools:
	@echo "=== Installing Go tools"
	cd ../tools && ./install.sh

# run go build with the version of this current rev-head.
# and run the server locally.
.PHONY: run
run:
	@echo "=== Building ems gRPC server"
	CGO_ENABLED=0 go build \
		-ldflags="-w -s -X main.version=local" \
		-o ./dist/ems-api . && ./dist/ems-api

# run go test with race detector
.PHONY: test
test:
	@echo "=== Running ems tests"
	CGO_ENABLED=1 \
		go test -vet=off -count=1 -race -timeout=5m ./...

# build container image
.PHONY: image
image:
	@echo "=== Building ems gRPC server container image"
	docker build \
		--build-arg GITHUB_TOKEN=$(GITHUB_TOKEN) \
		--build-arg VERSION=$$IMAGE_TAG \
		-t $$IMAGE_NAME .
