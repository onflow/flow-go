FROM golang:latest

RUN mkdir /bamboo-emulator

ADD . /bamboo-emulator

WORKDIR /bamboo-emulator

RUN go build

CMD ["./bamboo-emulator"]