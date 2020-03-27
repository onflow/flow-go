# Bootstrap

This package contains the CLIs to bootstrap a flow network.

WARNING: These scripts use Go's crypto/rand package to generate seeds for private keys. Make sure you are running the bootstrap scripts on a machine that does provide proper lower level implementations. See https://golang.org/pkg/crypto/rand/ for details.

NOTE: Public and private keys are encoded in JSON files as base64 strings, not hex, as might be expected.

## Usage

`go run -tags relic ./cmd/bootstrap` prints usage information

## Example Process

### Step 1: Generate networking and staking keys for partner nodes:

```bash
go run -tags relic ./cmd/bootstrap key --address "example.com" --role "consensus" --networking-seed d69b867d5932037c02a4f44335502138b56722adb07a8379ce6736fe4a0b9192443eb694bb3b7f18e0133d68f55a02a3997d6a163ce36280686cda3eba8524ca --staking-seed 23f2421dbcae62de1954b18bd6f4b96ca0aeeef90ea83d89aa542e727c7be78d0ed9a220b049b482cb3342c0534e429663f44d5d2c03ade73e74812489da884b -o ./bootstrap/partner-node-infos
```

### Step 2: Finalize the bootstrap process
```bash
go run -tags relic ./cmd/bootstrap finalize --config ./cmd/bootstrap/example_files/node-config.json --partner-dir ./cmd/bootstrap/example_files/partner-node-infos --partner-stakes ./cmd/bootstrap/example_files/partner-stakes.json
```

## Structure

`cmd/bootstrap/cmd` contains CLI logic that can exit the program and read/write files. It also uses structures and data types that are purely relevant for CLI purposes, such as encoding, decoding etc...
`cmd/bootstrap/run` contains reusable logic that does not know about a CLI. Instead of exiting a program, functions here will return errors.
