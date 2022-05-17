# gcr.io/dl-flow/golang-cmake

FROM golang:1.18-buster
RUN apt-get update
RUN apt-get -y install cmake zip
RUN go get github.com/axw/gocov/gocov
RUN go get github.com/matm/gocov-html
WORKDIR /go/src/flow
