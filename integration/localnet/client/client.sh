docker build -t localnet-client ./client
docker run --network host localnet-client /go/flow-cli-0.34.0/cmd/flow/flow version


