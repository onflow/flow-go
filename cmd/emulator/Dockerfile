# NOTE: Must be run in the context of the repo's root directory

# Build Relic first to maximize caching
FROM debian:buster AS build-relic

RUN apt-get update && apt-get install -y \
	build-essential \
	cmake

RUN mkdir /build
WORKDIR /build

# Copy over only files necessary for Relic
COPY pkg/crypto/relic ./relic
COPY pkg/crypto/relic_build.sh .

# Build Relic (this places build artifacts in /build/relic/build)
RUN ./relic_build.sh

FROM golang:1.13 AS build-app

RUN mkdir /build
WORKDIR /build

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
# Copy over Relic build artifacts
COPY --from=build-relic /build/relic/build ./pkg/crypto/relic/build

RUN GO111MODULE=on CGO_ENABLED=1 GOOOS=linux GOARCH=amd64 go build -o ./app ./cmd/emulator

FROM gcr.io/distroless/base

COPY --from=build-app /build/app /bin/app

# Expose HTTP and GRPC ports
EXPOSE 8000
EXPOSE 9090

# Start the emulator and initialize project config by generating a root key.
ENTRYPOINT ["/bin/app", "emulator", "start", "--init"]