# Example Go Project

This is a sample Go application that uses the Flow Go SDK to interact with the blockchain.

## Setup

If you are using Go 1.13, you have to set the `GOPRIVATE` environment variable to allow private packages from Dapper Labs:

```shell script
export GOPRIVATE=github.com/dapperlabs/*
```

## Testing

Run the sample tests:

```shell script
go test
```