# Bootstrapping 

This package contains script for generating genesis information to bootstrap the Flow network.
The high-level bootstrapping process is described in [Notion](https://www.notion.so/dapperlabs/Flow-Bootstrapping-ce9d227f18a8410dbce74ed7d4ddee27).
 
WARNING: These scripts use Go's crypto/rand package to generate seeds for private keys. Make sure you are running the bootstrap scripts on a machine that does provide proper lower level implementations. See https://golang.org/pkg/crypto/rand/ for details.

NOTE: Public and private keys are encoded in JSON files as base64 strings, not hex, as might be expected.

Code structure:
* `cmd/bootstrap/cmd` contains CLI logic that can exit the program and read/write files. It also uses structures and data types that are purely relevant for CLI purposes, such as encoding, decoding etc...
* `cmd/bootstrap/run` contains reusable logic that does not know about a CLI. Instead of exiting a program, functions here will return errors.



# Overview

The bootstrapping will generate the following information:

#### Per node
* Staking key (BLS key with curve BLS12381)
* Networking key (ECDSA key)
* Random beacon key; _only_ for consensus nodes (BLS based on Joint-Feldman DKG for threshold signatures)

#### Execution state
* Key for Account-Zero, i.e. the first account on Flow (ECDSA key)
* Genesis Execution state: LevelDB dump of execution state including pre-generated Account-Zero

#### Node Identities 
* List of all authorized Flow nodes 
  - node network address
  - node ID
  - node role
  - public staking key
  - public networking key
  - stake 

#### Root Block for main consensus 
* Genesis Block 
* Genesis QC: votes from consensus nodes for the genesis block (required to start consensus)

    
#### Root Blocks for Collector clusters
_Each cluster_ of collector nodes needs to have its own genesis Block and genesis QC 
* Genesis `ClusterBlockProposal` 
* Genesis QC from cluster for their respective `ClusterBlockProposal`


# Usage

`go run -tags relic ./cmd/bootstrap` prints usage information

## Phase 1: Generate networking and staking keys for partner nodes:

This step will generate the staking and networking keys for a _single partner node_.

#### Required Inputs
Values directly specified as command line parameters:
  - node network address
  - node role
  - seed for generating staking key (min 64 bytes in hex encoding)
  - seed for generating networking key (min 64 bytes in hex encoding)

#### Example
```bash
go run -tags relic ./cmd/bootstrap key -a "example.com" -r "consensus" --networking-seed d69b867d5932037c02a4f44335502138b56722adb07a8379ce6736fe4a0b9192443eb694bb3b7f18e0133d68f55a02a3997d6a163ce36280686cda3eba8524ca --staking-seed 23f2421dbcae62de1954b18bd6f4b96ca0aeeef90ea83d89aa542e727c7be78d0ed9a220b049b482cb3342c0534e429663f44d5d2c03ade73e74812489da884b -o ./bootstrap/partner-node-infos
```

#### Generated output files
* file `<NodeID>.node-info.priv.json`
   - strictly CONFIDENTIAL  (only for respective partner node with ID <NodeID>)
   - contains node's private staking and networking keys (plus some other auxiliary information) 
   - REQUIRED at NODE START;
     file needs to be available to respective partner node at boot up (or recovery after crash) 
* file `<NodeID>.node-info.pub.json`
   - public information 
   - file needs to be delivered to Dapper Labs for Phase 2 of generating genesis information,
     but is not required at node start 


### Phase 2: Generating final genesis information

This step will generate the entire genesis information for all nodes (incl. keys for all Dapper-controlled nodes).

#### Required Inputs
Each input is a config file specified as a command line parameter:
* `json` containing configuration for all Dapper-Controlled nodes (see `./example_files/node-config.json`)
* folder containing the `<NodeID>.node-info.pub.json` files for _all_ partner nodes (see `.example_files/partner-node-infos`)
* `json` containing the stake value for all partner nodes (see `./example_files/partner-stakes.json`). 
  Format: ```<NodeID>: <stake value>```

#### Example
```bash
go run -tags relic ./cmd/bootstrap finalize -c ./cmd/bootstrap/example_files/node-config.json --partner-dir ./cmd/bootstrap/example_files/partner-node-infos --partner-stakes ./cmd/bootstrap/example_files/partner-stakes.json -o ./bootstrap/genesis-infos
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


* file `account-0.priv.json`
   - strictly CONFIDENTIAL (only for Dapper Labs; not available to any node)
   - contains Account-Zero's private key!
   - file is _not_ required at node start by any node (and should not be accessible)
   - we could put this file into 1Password for now 
* folder `execution-state`
   - contains public LevelDB dump of execution state including pre-generated Account-Zero
   - REQUIRED at NODE START by all execution nodes
   - file can be made accessible to all nodes at boot up (or recovery after crash)
* file `genesis-block.json`
   - REQUIRED at NODE START by all nodes
* file `genesis-qc.json`
   - REQUIRED at NODE START by all nodes
* file `dkg-data.pub.json`
   - REQUIRED at NODE START by all nodes

* file `<ClusterID>-genesis-cluster-block.json`
   - genesis `ClusterBlockProposal` for collector cluster with ID `<ClusterID>`
   - REQUIRED at NODE START by all collectors of the respective cluster
   - file can be made accessible to all nodes at boot up (or recovery after crash)
* file `<ClusterID>-genesis-cluster-qc.json`
   - genesis Quorum Certificate for `ClusterBlockProposal` for collector cluster with ID `<ClusterID>`
   - REQUIRED at NODE START by all collectors of the respective cluster
   - file can be made accessible to all nodes at boot up (or recovery after crash)
