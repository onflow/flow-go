# Bootstrap

This package contains the CLIs to bootstrap a flow network.

WARNING: These scripts use Go's crypto/rand package to generate seeds for private keys. Make sure you are running the bootstrap scripts on a machine that does provide proper lower level implementations. See https://golang.org/pkg/crypto/rand/ for details.

## Example Process

Step 1: TODO
Step 2: Finalize the bootstrap process `go run -tags relic ./cmd/bootstrap finalize -c ./cmd/bootstrap/example_files/node_config.yml`
