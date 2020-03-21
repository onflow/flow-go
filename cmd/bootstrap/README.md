# Bootstrap

This package contains the CLIs to bootstrap a flow network.

WARNING: These scripts use Go's crypto/rand package to generate seeds for private keys. Make sure you are running the bootstrap scripts on a machine that does provide proper lower level implementations. See https://golang.org/pkg/crypto/rand/ for details.

NOTE: Public and private keys are encoded in JSON files as base64 strings, not hex, as might be expected.

## Usage

`go run -tags relic ./cmd/bootstrap` prints usage information

## Example Process

Step 1: TODO
Step 2: Finalize the bootstrap process `go run -tags relic ./cmd/bootstrap finalize -c ./cmd/bootstrap/example_files/node_config.json`

## Structure

`cmd/bootstrap/cmd` contains CLI logic that can exit the program and read/write files. It also uses structures and data types that are purely relevant for CLI purposes, such as encoding, decoding etc...
`cmd/bootstrap/run` contains reusable logic that does not know about a CLI. Instead of exiting a program, functions here will return errors.
