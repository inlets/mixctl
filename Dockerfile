FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.19 as builder

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

ARG GIT_COMMIT
ARG VERSION

ENV GO111MODULE=on
ENV GOFLAGS=-mod=vendor
ENV CGO_ENABLED=0

WORKDIR /usr/bin/

WORKDIR /go/src/github.com/inlets/mixctl
COPY . .

# Run a gofmt and exclude all vendored code.
RUN test -z "$(gofmt -l $(find . -type f -name '*.go' -not -path "./vendor/*"))" || { echo "Run \"gofmt -s -w\" on your Golang code"; exit 1; }

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 \
    go build --ldflags "-s -w \
    -X github.com/inlets/mixctl/version.GitCommit=${GIT_COMMIT} \
    -X github.com/inlets/mixctl/version.Version=${VERSION} \
    -X github.com/inlets/mixctl/version.Platform=${TARGETARCH}" \
    -a -installsuffix netgo -o /usr/bin/mixctl

FROM --platform=${TARGETPLATFORM:-linux/amd64} scratch as release
COPY --from=builder /usr/bin/mixctl /

ENTRYPOINT ["/mixctl"]
