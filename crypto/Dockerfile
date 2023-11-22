# gcr.io/dl-flow/golang-cmake

FROM golang:1.20-buster
RUN apt-get update
RUN apt-get -y install cmake zip
RUN go install github.com/axw/gocov/gocov@latest
RUN go install github.com/matm/gocov-html@latest
WORKDIR /go/src/flow
