Version := $(shell git describe --tags --dirty)
GitCommit := $(shell git rev-parse HEAD)
LDFLAGS := "-s -w -X github.com/inlets/mixctl/main.Version=$(Version) -X github.com/inlets/mixctl/main.GitCommit=$(GitCommit)"
PLATFORM := $(shell ./hack/platform-tag.sh)
SOURCE_DIRS = main.go
export GO111MODULE=on

IMG_NAME?=mixctl
TAG?=latest
OWNER?=alexellis2
SERVER?=docker.io

.PHONY: publish
publish:
	@echo  $(SERVER)/$(OWNER)/$(IMG_NAME):$(TAG) && \
	docker buildx create --use --name=multiarch --node=multiarch && \
	docker buildx build \
		--platform linux/amd64,linux/arm/v7,linux/arm64,darwin/amd64,darwin/arm64 \
		--push=true \
        --build-arg GIT_COMMIT=$(GIT_COMMIT) \
        --build-arg VERSION=$(VERSION) \
		--tag $(SERVER)/$(OWNER)/$(IMG_NAME):$(TAG) \
		--no-cache \
		.

.PHONY: all
all: gofmt test build dist hash

.PHONY: build
build:
	go build

.PHONY: gofmt
gofmt:
	@test -z $(shell gofmt -l -s $(SOURCE_DIRS) ./ | tee /dev/stderr) || (echo "[WARN] Fix formatting issues with 'make gofmt'" && exit 1)

.PHONY: test
test:
	CGO_ENABLED=0 go test $(shell go list ./... | grep -v /vendor/|xargs echo) -cover

.PHONY: dist
dist:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags $(LDFLAGS) -a -installsuffix netgo -o bin/mixctl
	CGO_ENABLED=0 GOOS=darwin go build -ldflags $(LDFLAGS) -a -installsuffix netgo -o bin/mixctl-darwin
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -a -ldflags $(LDFLAGS) -installsuffix netgo -o bin/mixctl-darwin-arm64
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build -ldflags $(LDFLAGS) -a -installsuffix netgo -o bin/mixctl-armhf
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags $(LDFLAGS) -a -installsuffix netgo -o bin/mixctl-arm64
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags $(LDFLAGS) -a -installsuffix netgo -o bin/mixctl.exe

.PHONY: hash
hash:
	rm -rf bin/*.sha256 && ./hack/hashgen.sh
