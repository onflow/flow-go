# syntax = docker/dockerfile:experimental
# NOTE: Must be run in the context of the repo's root directory

FROM golang:1.18-bullseye AS build-setup

RUN apt-get update
RUN apt-get -y install cmake zip

## (2) Build the app binary
FROM build-setup AS build-env

# add the pubkey of github.com to knownhosts, so ssh-agent doesn't bark
RUN mkdir -p /root/.ssh && ssh-keyscan -t rsa github.com >> /root/.ssh/known_hosts
RUN git config --global 'url.ssh://git@github.com/.insteadof' https://github.com/

# Build the app binary in /app
RUN mkdir /app
WORKDIR /app

ARG TARGET
ARG COMMIT
ARG VERSION

COPY . .

RUN --mount=type=ssh \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    make crypto_setup_gopath

FROM build-env as build-production
WORKDIR /app
ARG GOARCH=amd64
# Keep Go's build cache between builds.
# https://github.com/golang/go/issues/27719#issuecomment-514747274
RUN --mount=type=ssh \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GO111MODULE=on CGO_ENABLED=1 GOOS=linux go build --tags "relic,netgo" -ldflags "-extldflags -static \
    -X 'github.com/onflow/flow-go/cmd/build.commit=${COMMIT}' -X  'github.com/onflow/flow-go/cmd/build.semver=${VERSION}'" \
    -o ./app ${TARGET}

RUN chmod a+x /app/app

## (3) Add the statically linked binary to a distroless image
FROM gcr.io/distroless/base-debian11 as production

COPY --from=build-production /app/app /bin/app

ENTRYPOINT ["/bin/app"]


FROM build-env as build-debug
WORKDIR /app
ARG GOARCH=amd64
RUN --mount=type=ssh \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GO111MODULE=on CGO_ENABLED=1 GOOS=linux go build --tags "relic,netgo" -ldflags "-extldflags -static \
    -X 'github.com/onflow/flow-go/cmd/build.commit=${COMMIT}' -X  'github.com/onflow/flow-go/cmd/build.semver=${VERSION}'" \
    -gcflags="all=-N -l" -o ./app ${TARGET}

RUN chmod a+x /app/app

FROM golang:1.18-bullseye as debug

RUN go get -u github.com/go-delve/delve/cmd/dlv

COPY --from=build-debug /app/app /bin/app

ENTRYPOINT ["dlv", "--listen=:2345", "--headless=true", "--api-version=2", "--accept-multiclient", "exec", "/bin/app", "--"]

## (3) Add the statically linked binary to a distroless image
FROM gcr.io/distroless/base-debian11 as production-transit-nocgo

COPY --from=build-transit-production-nocgo /app/app /bin/app

ENTRYPOINT ["/bin/app"]
