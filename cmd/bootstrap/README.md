# Bootstrapping

This package contains script for generating the bootstrap files needed to initialize the Flow network.
The high-level bootstrapping process is described in [Notion](https://www.notion.so/dapperlabs/Flow-Bootstrapping-ce9d227f18a8410dbce74ed7d4ddee27).

WARNING: These scripts use Go's crypto/rand package to generate seeds for private keys. Make sure you are running the bootstrap scripts on a machine that does provide proper a low-level implementation. See https://golang.org/pkg/crypto/rand/ for details.

NOTE: Public and private keys are encoded in JSON files as base64 strings, not as hex, contrary to what might be expected.

Code structure:
* `cmd/bootstrap/cmd` contains CLI logic that can exit the program and read/write files. It also uses structures and data types that are purely relevant for CLI purposes, such as encoding, decoding, etc.
* `cmd/bootstrap/run` contains reusable logic that does not know about the CLI. Instead of exiting the program, functions here will return errors.



# Overview

The bootstrapping will generate the following information:

#### Per node
* Staking key (BLS key with curve BLS12-381)
* Networking key (ECDSA key)
* Random beacon key; _only_ for consensus nodes (BLS based on Joint-Feldman DKG for threshold signatures)

#### Node Identities
* List of all authorized Flow nodes
  - node network address
  - node ID
  - node role
  - public staking key
  - public networking key
  - weight

#### Root Block for main consensus
* Root Block
* Root QC: votes from consensus nodes for the root block (required to start consensus)
* Root Execution Result: execution result for the initial execution state
* Root Block Seal: block seal for the initial execution result


#### Root Blocks for Collector clusters
_Each cluster_ of collector nodes needs to have its own root Block and root QC
* Root `ClusterBlockProposal`
* Root QC from cluster for their respective `ClusterBlockProposal`


# Usage

`go run -tags relic ./cmd/bootstrap` prints usage information

## Phase 1: Generate networking and staking keys for partner nodes:

This step will generate the staking and networking keys for a _single partner node_.

#### Required Inputs
Values directly specified as command line parameters:
  - node network address
  - node role

#### Optional Inputs
Values can be specified as command line parameters:
  - seed for generating staking key (min 48 bytes in hex encoding)
  - seed for generating networking key (min 48 bytes in hex encoding)
If seeds are not provided, the CLI will try to use the system's pseudo-random number generator (PRNG), e. g. `dev/urandom`. Make sure you are running the CLI on a hardware that has a cryptographically secure PRNG, or provide seeds generated on such a system.

#### Example
```bash
go run -tags relic ./cmd/bootstrap key --address "example.com:1234" --role "consensus" -o ./bootstrap/partner-node-infos
```

#### Generated output files
* file `<NodeID>.node-info.priv.json`
   - strictly CONFIDENTIAL  (only for respective partner node with ID <NodeID>)
   - contains node's private staking and networking keys (plus some other auxiliary information)
   - REQUIRED at NODE START;
     file needs to be available to respective partner node at boot up (or recovery after crash)
* file `<NodeID>.node-info.pub.json`
   - public information
   - file needs to be delivered to Dapper Labs for Phase 2 of generating root information,
     but is not required at node start


### Phase 2: Generating final root information

This step will generate the entire root information for all nodes (incl. keys for all Dapper-controlled nodes).

#### Required Inputs
Each input is a config file specified as a command line parameter:
* parameter with the ID for the chain for the root block (`root-chain`)
* parameter with the ID of the parent block for the root block (`root-parent`)
* parameter with height of the root block to bootstrap from (`root-height`)
* parameter with state commitment for the initial execution state (`root-commit`)
* `json` containing configuration for all Dapper-Controlled nodes (see `./example_files/node-config.json`)
* folder containing the `<NodeID>.node-info.pub.json` files for _all_ partner nodes (see `.example_files/partner-node-infos`)
* `json` containing the weight value for all partner nodes (see `./example_files/partner-weights.json`).
  Format: ```<NodeID>: <weight value>```

#### Example
```bash
go run -tags relic ./cmd/bootstrap finalize \
 --fast-kg \
  --root-chain main \
  --root-height 0 \
  --root-parent 0000000000000000000000000000000000000000000000000000000000000000 \
  --root-commit 4b8d01975cf0cd23e046b1fae36518e542f92a6e35bedd627c43da30f4ae761a \
  --config ./cmd/bootstrap/example_files/node-config.json \
  --partner-dir ./cmd/bootstrap/example_files/partner-node-infos \
  --partner-weights ./cmd/bootstrap/example_files/partner-weights.json \
  --epoch-counter 1 \
  -o ./bootstrap/root-infos
```

#### Generated output files
* files `<NodeID>.node-info.priv.json`
   - strictly CONFIDENTIAL (only for respective Dapper node with ID <NodeID>)
   - contains node's private staking and networking keys (plus some other auxiliary information)
   - REQUIRED at NODE START:
     file needs to be available to respective Dapper node at boot up (or recovery after crash)
* files `<NodeID>.random-beacon.priv.json`
   - strictly CONFIDENTIAL (only for consensus node with ID <NodeID>)
   - CAUTION: we generate the random beacon private keys for _all_ consensus nodes, i.e. Dapper and Partner nodes alike!
     The private random beacon keys must be delivered to the Partner Node operator securely.
   - contains node's _private_ random beacon key
   - REQUIRED at NODE START:
     file needs to be available to respective consensus node at boot up (or recovery after crash)
* file `node-infos.pub.json`
   - contains public Node Identities for all authorized Flow nodes (Dapper and Partner nodes)
   - REQUIRED at NODE START for all nodes;
     file needs to be available to all nodes at boot up (or recovery after crash)

* file `root-block.json`
   - REQUIRED at NODE START by all nodes
* file `root-qc.json`
   - REQUIRED at NODE START by all nodes
* file `root-result.json`
   - REQUIRED at NODE START by all nodes
* file `root-seal.json`
   - REQUIRED at NODE START by all nodes
* file `dkg-data.pub.json`
   - REQUIRED at NODE START by all nodes

* file `<ClusterID>.root-cluster-block.json`
   - root `ClusterBlockProposal` for collector cluster with ID `<ClusterID>`
   - REQUIRED at NODE START by all collectors of the respective cluster
   - file can be made accessible to all nodes at boot up (or recovery after crash)
* file `<ClusterID>.root-cluster-qc.json`
   - root Quorum Certificate for `ClusterBlockProposal` for collector cluster with ID `<ClusterID>`
   - REQUIRED at NODE START by all collectors of the respective cluster
   - file can be made accessible to all nodes at boot up (or recovery after crash)
