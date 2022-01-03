# syntax = docker/dockerfile:experimental
# NOTE: Must be run in the context of the repo's root directory

FROM golang:1.17-buster AS build-setup

RUN apt-get update
RUN apt-get -y install cmake zip sudo

## (1) Build Relic first to maximize caching
FROM build-setup AS build-relic

RUN mkdir /build
WORKDIR /build

# Copy over the crypto package
COPY crypto ./crypto

# Build Relic (this places build artifacts in /build/relic/build)
RUN cd ./crypto/ && go generate

## (2) Build the app binary
FROM build-setup AS build-env

# Build the app binary in /app
RUN mkdir /app
WORKDIR /app

ARG TARGET

COPY . .

# Copy over Relic build artifacts
COPY --from=build-relic /build/crypto/relic/build ./crypto/relic/build

FROM build-env as build-production
WORKDIR /app

# Keep Go's build cache between builds.
# https://github.com/golang/go/issues/27719#issuecomment-514747274
# Also, allow ssh access
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=ssh \
    cd integration && \
    GO111MODULE=on CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build --tags relic -ldflags "-extldflags -static" -o ./app ./${TARGET}

RUN mv /app/integration/app /app/app

RUN chmod a+x /app/app

## (3) Add the statically linked binary to a distroless image
FROM gcr.io/distroless/base as production

COPY --from=build-production /app/app /bin/app

ENTRYPOINT ["/bin/app"]
