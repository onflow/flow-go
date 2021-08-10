# gcr.io/dl-flow/golang-cmake

FROM golang:1.15-buster
RUN apt-get update
RUN apt-get -y install cmake zip sudo
RUN go get github.com/axw/gocov/gocov
RUN go get github.com/matm/gocov-html
RUN echo '%sudo ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers
WORKDIR /go/src/flow
