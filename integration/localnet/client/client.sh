docker build -t localnet-client ./client
docker run --network host localnet-client /go/flow version


