# Execution

The Execution Node (EN) processes blocks and collections prepared by consensus and collection
nodes. It executes the transaction in a Flow Virtual Machine, runtime environment of Flow protocol.
EN also maintains Execution State, ledger containing user data which is manipulated by transactions.
As a last step of processing block, it prepares and distributes Execution Receipt allowing verification
nodes to verify correctness of computation, and later, consensus nodes to seal blocks.
 
<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
##Table of Contents

- [Terminology](#terminology)
- [Architecture overview](#architecture-overview)
  - [Ingestion engine](#ingestion-engine)
  - [ComputationManager](#computationmanager)
  - [Provider engine](#provider-engine)
  - [RPC Engine](#rpc-engine)
- [Ingestion operation](#ingestion-operation)
  - [Mempool queues](#mempool-queues)
  - [Mempool cache](#mempool-cache)
- [Syncing](#syncing)
  - [Execution State syncing](#execution-state-syncing)
  - [Missing blocks](#missing-blocks)
- [Operation](#operation)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Terminology

- Computation - is a process of evaluating block content and yielding list of changes to the ledger.  
- Execution - complete, Flow protocol compliant operation of executing block. It manages environment for computation of block, stores execution states, creates execution receipt among others.

## Architecture overview

### Ingestion engine
Central component - input of the node - execution state owner, only component allowed to mutate it. 
It receives blocks, assembles them (gather collections) and forms execution queue (fork-aware ordering).
Once a block is assembled and ready to be computed (full content of transactions is known and valid, previous execution state is available) it passes `ExecutableBlock` for computation (to `ComputationManager`)
After receiving computation results ingestion engine generates Flow-compliant entities (`ExecutionReceipt`) and saves them along with other internal execution state data (block's `StateCommitment`).

### ComputationManager
Abstraction of computing blocks, creates and manages Cadence runtime, exposes interface for computing blocks.

### Provider engine
The output of the Execution Node. It's responsible for broadcasting `ExecutionReceipts` and answering requests for various states of protocol.

### RPC Engine
It's gRPC endpoint exposing Observation API. This is a temporary solution and Observation Node is expected to take over this responsibility.

## Ingestion operation

### Mempool queues
Ingestion engine accepts incoming blocks and classifies them into two mempool map of queues:
 - execution queue - which contains subqueues of blocks executable in order. This allow forks to be executed in parallel. 
   Head of each queue is either being executed or waiting for collections. It's starting state commitment is present. It will
   produce state commitment for it's children. 
 - orphan queue - which contains subqueues of orphaned blocks. Those who cannot be executed immediately or are not known to be
   executable soon. It's kept separately, as it will be used to determine when the node should switch into synchronisation mode
   
### Mempool cache
Additionally, EN keeps a simple mapping of collection IDs to block, for lookup when the collection is received.

## Syncing

If EN cannot execute number of consecutive blocks (`syncThreshold` parameter) it enter synchronisation mode. The number of blocks
required to trigger this condition is put in place to prevent triggering it in case of blocks arriving out of order, which can
happen on unstable networks.
EN keeps track of the highest block it has executed. This is not a Flow protocol feature, and only serves synchronisation needs.

### Execution State syncing
Other execution node is queried for range of missing blocks and hold authority to decide if it's willing (and able) to answer this query.
If so, it sends the `ExecutionStateDelta` which contains all the block data and results of execution.
Currently, this is fully trusted operation, meaning data is applied as-is without any extra checks.

### Missing blocks
If no other EN are available, the block-level synchronisation is started. This requests blocks from consensus nodes, and
incoming blocks are processed as if they were received during normal mode of operation

## Operation

In order to execute a block, all collections must be requested and validated. A valid collection must be signed by an authorized collection node (i.e. with positive weight). The protocol state can be altered by executing transactions, hence the parent block must be executed to provide
up-to-date copy of protocol state. This allows to validate collection nodes identities and in turn, validity of collection itself.
Having all collections retrieved and Execution State of a parent known - a block execution can commence.
Blocks are executed in separate Go routine to allow potential forks (sharing parent block) to be computed in parallel.
After execution is finished, it passes newly created execution state to its children, and if they are now ready - they are, repeating the loop.

