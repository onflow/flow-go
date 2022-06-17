FROM golang:1.17

COPY flow-localnet.json /go

WORKDIR /go
RUN curl -L https://github.com/onflow/flow-cli/archive/refs/tags/v0.34.0.tar.gz | tar -xzv
RUN cd flow-cli-0.34.0 && go mod download 
RUN cd flow-cli-0.34.0 && make
RUN /go/flow-cli-0.34.0/cmd/flow/flow version

CMD /go/flow-cli-0.34.0/cmd/flow/flow -f /go/flow-localnet.json -n observer blocks get latest
