# Bootstrap

This package contains the CLIs to bootstrap a flow network.

WARNING: These scripts use Go's crypto/rand package to generate seeds for private keys. Make sure you are running the bootstrap scripts on a machine that does provide proper lower level implementations. See https://golang.org/pkg/crypto/rand/ for details.

NOTE: Public and private keys are encoded in JSON files as base64 strings, not hex, as might be expected.

## Usage

`go run -tags relic ./cmd/bootstrap` prints usage information

## Example Process

### Step 1: Generate networking and staking keys for partner nodes:

```bash
go run -tags relic ./cmd/bootstrap key -a "example.com" -r "consensus" --networking-seed c06ffd55a8515e8bea7db873609e998f98eefbb7e9c185494c65cae567012788ee4eaf41
706ffbac59f22f1fd81822c6877422f5d754a62bdbbce24c8382a74c --staking-seed 219cd6452c410c1518fe3afc1d2f6b78658a583495846d430d6c93baf9f92028213d407129cdaa0eea1f184c84a5f0bde6c5afa925b8e5a
477b8ebe4cdd42384
```

### Step 2: Finalize the bootstrap process
```bash
go run -tags relic ./cmd/bootstrap finalize -c ./cmd/bootstrap/example_files/node_config.json
```

## Structure

`cmd/bootstrap/cmd` contains CLI logic that can exit the program and read/write files. It also uses structures and data types that are purely relevant for CLI purposes, such as encoding, decoding etc...
`cmd/bootstrap/run` contains reusable logic that does not know about a CLI. Instead of exiting a program, functions here will return errors.
