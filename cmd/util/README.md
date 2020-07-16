# Util

This package contains utilities that access a node's database.

## Usage

Run `go run ./cmd/util` for usage details.


## Commands

### execution-state-extract
Commands which reads WAL of Execution Node state from `execution-state-dir`, until it finds a State Commitment
matching given `block-hash`. It then creates a checkpoint file (`root.checkpoint`) in `output-dir`. 
This is Execution State at this block (with some historical states, according to cache length).

Useful for sporking the network.

Content of `output-dir` shall be used as Execution Node state directory to boot EN.

Command should also print state commitment.
