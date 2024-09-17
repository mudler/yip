NAME ?= yip
PACKAGE_NAME ?= $(NAME)
PACKAGE_CONFLICT ?= $(PACKAGE_NAME)-beta
REVISION := $(shell git rev-parse --short HEAD || echo dev)
VERSION := $(shell git describe --tags || echo $(REVISION))
VERSION := $(shell echo $(VERSION) | sed -e 's/^v//g')
ITTERATION := $(shell date +%s)
BUILD_PLATFORMS ?= -osarch="linux/amd64" -osarch="linux/386" -osarch="linux/arm"
ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

# go tool nm ./yip | grep Commit
override LDFLAGS += -X "github.com/mudler/yip/cmd.BuildTime=$(shell date -u '+%Y-%m-%d %I:%M:%S %Z')"
override LDFLAGS += -X "github.com/mudler/yip/cmd.BuildCommit=$(shell git rev-parse HEAD)"

.PHONY: all
all: build

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: test
test:
	go mod tidy -compat=1.22
	go run github.com/onsi/ginkgo/v2/ginkgo -race -r ./...

.PHONY: coverage
coverage:
	go run github.com/onsi/ginkgo/v2/ginkgo -race -coverprofile=coverage.txt -covermode=atomic ./...

.PHONY: help
help:
	# make all => test lint build
	# make test - run project tests
	# make lint - check project code style
	# make build - build project for all supported OSes

.PHONY: clean
clean:
	rm -rf release/

.PHONY: build
build:
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)'

.PHONY: build-small
build-small:
	@$(MAKE) LDFLAGS+="-s -w" build
	upx --brute -1 $(NAME)

.PHONY: lint
lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.52.2
	golangci-lint run
