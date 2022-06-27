FROM golang:1.17

COPY flow-localnet.json /go

WORKDIR /go
RUN curl -L https://github.com/onflow/flow-cli/archive/refs/tags/v0.36.2.tar.gz | tar -xzv
RUN cd flow-cli-0.36.2 && go mod download
RUN cd flow-cli-0.36.2 && make
RUN /go/flow-cli-0.36.2/cmd/flow/flow version
RUN cp /go/flow-cli-0.36.2/cmd/flow/flow /go/flow


CMD /go/flow -f /go/flow-localnet.json -n observer blocks get latest
