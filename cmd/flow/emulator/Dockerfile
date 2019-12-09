# NOTE: Must be run in the context of the repo's root directory

## (2) Build the app binary
FROM golang:1.13-alpine3.10 AS build-app

# Build the app binary in /app
RUN mkdir /app
WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

RUN GO111MODULE=on CGO_ENABLED=0 GOOOS=linux GOARCH=amd64 go build -o ./app ./cmd/flow

## (3) Add the binary to a fresh Alpine image
FROM alpine:3.10

COPY --from=build-app /app/app /bin/app

# Expose GRPC and HTTP ports
EXPOSE 8080
EXPOSE 3569

# Run the CLI binary as the entrypoint
ENTRYPOINT ["/bin/app"]
# These arguments are separated from the entrypoint to simplify running other
# commands with this image.
CMD ["emulator", "start", "--init", "--persistent"]
