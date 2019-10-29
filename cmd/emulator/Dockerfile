# NOTE: Must be run in the context of the repo's root directory

## (1) Build Relic first to maximize caching
FROM golang:1.13-alpine3.10 AS build-relic

RUN apk update && apk add \
    build-base \
    cmake \
    bash

RUN mkdir /build
WORKDIR /build

# Copy over *only* files necessary for Relic
COPY crypto/relic ./relic
COPY crypto/relic_build.sh .

# Build Relic (this places build artifacts in /build/relic/build)
# NOTE: The Relic build script uses Bash-specific features, so we explicitly run
# it with bash rather than the default shell.
RUN bash ./relic_build.sh

## (2) Build the app binary
FROM golang:1.13-alpine3.10 AS build-app

RUN apk update && apk add build-base

# Build the app binary in /app
RUN mkdir /app
WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
# Copy over Relic build artifacts
COPY --from=build-relic /build/relic/build ./crypto/relic/build

RUN GO111MODULE=on CGO_ENABLED=1 GOOOS=linux GOARCH=amd64 go build -o ./app ./cmd/emulator

## (3) Add the binary to a fresh Alpine image
FROM alpine:3.10

COPY --from=build-app /app/app /bin/app

# Expose HTTP and GRPC ports
EXPOSE 8000
EXPOSE 9090

# Start the emulator and initialize project config by generating a root key.
ENTRYPOINT ["/bin/app", "emulator", "start", "--init"]
