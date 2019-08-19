# NOTE: this Dockerfile must be run in the context of the top level repo folder
FROM golang:1.12 as build-env

RUN apt-get update && apt-get install -y \
	build-essential \
	cmake

RUN mkdir /app
WORKDIR /app

COPY go.mod .	
COPY go.sum .

RUN go mod download

COPY . .

RUN make build-relic
RUN GO111MODULE=on CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /go/bin/app ./cmd/execute/

FROM alpine
RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2
COPY --from=build-env /go/bin/app /go/bin/app

WORKDIR /go/bin
ENTRYPOINT ["./app"]
