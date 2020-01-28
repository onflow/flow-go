# syntax = docker/dockerfile:experimental
# NOTE: Must be run in the context of the repo's root directory

## (1) Build Relic first to maximize caching
FROM golang:1.13 AS build-relic

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
FROM golang:1.13 AS build-app

# add the pubkey of github.com to knownhosts, so ssh-agent doesn't bark
RUN mkdir -p /root/.ssh && ssh-keyscan -t rsa github.com >> /root/.ssh/known_hosts
RUN echo '[url "ssh://git@github.com/"] insteadOf = https://github.com/' > ~/.gitconfig

# Build the app binary in /app
RUN mkdir /app
WORKDIR /app

ARG TARGET

COPY . .

# Copy over Relic build artifacts
COPY --from=build-relic /build/relic/build ./crypto/relic/build

# Keep Go's build cache between builds.
# https://github.com/golang/go/issues/27719#issuecomment-514747274
# Also, allow ssh access
RUN --mount=type=ssh \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GO111MODULE=on CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags "-extldflags -static" -o ./app ./cmd/${TARGET}

## (3) Add the binary to a fresh Alpine image
FROM alpine:3.10

COPY --from=build-app /app/app /bin/app
RUN chmod a+x /bin/app

ENTRYPOINT ["/bin/app"]
