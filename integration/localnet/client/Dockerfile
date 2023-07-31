FROM golang:1.20

COPY flow-localnet.json /go

WORKDIR /go
RUN curl -L https://github.com/onflow/flow-cli/archive/refs/tags/v1.3.3.tar.gz | tar -xzv
RUN cd flow-cli-1.3.3 && go mod download
RUN cd flow-cli-1.3.3 && make
RUN /go/flow-cli-1.3.3/cmd/flow/flow version
RUN cp /go/flow-cli-1.3.3/cmd/flow/flow /go/flow


CMD /go/flow -f /go/flow-localnet.json -n observer blocks get latest
