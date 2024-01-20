# syntax = docker/dockerfile:experimental
# NOTE: Must be run in the context of the repo's root directory

####################################
## (1) Setup the build environment
FROM golang:1.20-bullseye AS build-setup

RUN apt-get update
RUN apt-get -y install zip

## (2) Setup crypto dependencies
FROM build-setup AS build-env

# Build the app binary in /app
RUN mkdir /app
WORKDIR /app

ARG TARGET
ARG COMMIT
ARG VERSION

ENV GOPRIVATE=

COPY . .

# Update the git config to use SSH rather than HTTPS for clones
RUN git config --global url.git@github.com:.insteadOf https://github.com/
RUN mkdir ~/.ssh

# Add GitHub known host
RUN ssh-keyscan -t rsa github.com >> ~/.ssh/known_hosts

####################################
## (3) Build the production app binary
FROM build-env as build-production
WORKDIR /app

ARG GOARCH=amd64

# TAGS can be overriden to modify the go build tags (e.g. build without netgo)
ARG TAGS="netgo"
# CGO_FLAG can be overwritten
ARG CGO_FLAG

# Keep Go's build cache between builds.
# https://github.com/golang/go/issues/27719#issuecomment-514747274
RUN --mount=type=cache,sharing=locked,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=secret,id=cadence_deploy_key \
    eval `ssh-agent -s` && printf "%s\n" "$(cat /run/secrets/cadence_deploy_key)" | ssh-add - ; \
    CGO_ENABLED=1 GOOS=linux CGO_FLAGS="${CGO_FLAG}" go build --tags "${TAGS}" -ldflags "-extldflags -static \
    -X 'github.com/onflow/flow-go/cmd/build.commit=${COMMIT}' -X  'github.com/onflow/flow-go/cmd/build.semver=${VERSION}'" \
    -o ./app ${TARGET}

RUN chmod a+x /app/app

## (4) Add the statically linked production binary to a distroless image
FROM gcr.io/distroless/base-debian11 as production

COPY --from=build-production /app/app /bin/app

ENTRYPOINT ["/bin/app"]

####################################
## (3) Build the debug app binary
FROM build-env as build-debug
WORKDIR /app
ARG GOARCH=amd64

RUN --mount=type=ssh \
    --mount=type=cache,sharing=locked,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=secret,id=cadence_deploy_key \
    eval `ssh-agent -s` && printf "%s\n" "$(cat /run/secrets/cadence_deploy_key)" | ssh-add - ; \
    CGO_ENABLED=1 GOOS=linux CGO_FLAGS="${CGO_FLAG}" go build --tags "netgo" -ldflags "-extldflags -static \
    -X 'github.com/onflow/flow-go/cmd/build.commit=${COMMIT}' -X  'github.com/onflow/flow-go/cmd/build.semver=${VERSION}'" \
    -gcflags="all=-N -l" -o ./app ${TARGET}

RUN chmod a+x /app/app

## (4) Add the statically linked debug binary to a distroless image configured for debugging
FROM golang:1.20-bullseye as debug

RUN go install github.com/go-delve/delve/cmd/dlv@latest

COPY --from=build-debug /app/app /bin/app

ENTRYPOINT ["dlv", "--listen=:2345", "--headless=true", "--api-version=2", "--accept-multiclient", "exec", "/bin/app", "--"]


FROM build-setup as environment-clean
