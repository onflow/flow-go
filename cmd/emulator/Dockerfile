# NOTE: Must be run in the context of the root repo directory
FROM golang:1.13 AS build-env

RUN apt-get update && apt-get install -y \
	build-essential \
	cmake

RUN mkdir /build
WORKDIR /build

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN make build-relic
RUN GO111MODULE=on CGO_ENABLED=1 GOOOS=linux GOARCH=amd64 go build -o ./app ./cmd/emulator

FROM gcr.io/distroless/base

COPY --from=build-env /build/app /bin/app

# Export HTTP and GRPC ports
EXPOSE 8000
EXPOSE 9090

# Start the emulator and initialize project config by generating a root key.
ENTRYPOINT ["/bin/app", "emulator", "start", "--init"]