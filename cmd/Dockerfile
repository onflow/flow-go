# syntax = docker/dockerfile:experimental
# NOTE: Must be run in the context of the repo's root directory

####################################
## (1) Setup the build environment
FROM golang:1.20-bullseye AS build-setup

RUN apt-get update
RUN apt-get -y install zip apt-utils gcc-aarch64-linux-gnu

## (2) Setup crypto dependencies
FROM build-setup AS build-env

# Build the app binary in /app
RUN mkdir /app
WORKDIR /app

COPY . .


####################################
## (3) Build the production app binary
FROM build-env as build-production
WORKDIR /app

# Keep Go's build cache between builds.
# https://github.com/golang/go/issues/27719#issuecomment-514747274
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC="" CGO_CFLAGS="-O2 -D__BLST_PORTABLE__" go build -ldflags "-extldflags -static" \
    -o ./app ./cmd/consensus

RUN chmod a+x /app/app

## (4) Add the statically linked production binary to a distroless image
FROM gcr.io/distroless/base-debian11 as production

COPY --from=build-production /app/app /bin/app

ENTRYPOINT ["/bin/app"]

FROM build-setup as environment-clean
