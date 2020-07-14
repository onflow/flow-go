# gcr.io/dl-flow/golang-cmake

FROM golang:1.13
RUN apt-get update
RUN apt-get -y install cmake clang zip sudo
RUN go get github.com/axw/gocov/gocov
RUN go get github.com/matm/gocov-html
RUN echo '%sudo ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers
RUN mkdir -p /root/.ssh && ssh-keyscan -t rsa github.com >> /root/.ssh/known_hosts
RUN git config --global --add 'url.ssh://git@github.com/.insteadof' https://github.com/
WORKDIR /go/src/flow
