# Execution node overview

### Terminology

- Computation - is a process of evaluating block content and yielding list of changes.  
- Execution - complete, Flow protocol compliant operation of executing block. It manages environment for computation of block, stores execution states, creates execution receipt among others.

## Architecture overview

### Ingestion engine
Central component - input of the node - execution state owner, only component allowed to mutate it. 
It receives blocks, assembles them (gather collections) and forms execution queue (fork-aware ordering).
Once a block is assembled and ready to be computed (full content of transaction is known and valid, previous execution state is available) it sends `CompleteBlock` for computation (to Computation engine)
After receiving computation results ingestion engine generates Flow-compliant entities (`ExecutionReceipt`) and saves them along with other internal execution state data (block's `StateCommitment`).

### Computation engine
Abstraction of computing blocks, creates and manages Cadence runtime, exposes interface for computing blocks.
Does NOT implement `network.Engine` interface. 

### (TBD) Scheduler
As an optimisation, blocks should be passed to a scheduler which should allow parallel execution of blocks when possible. 
Due to abstract nature of Computation component further optimisation might be possible, but this is not essential in current state.

### Provider engine
The output of the Execution Node. It's responsible for broadcasting `ExecutionReceipts` and answering requests for various states of protocol.

### RPC Engine
It's gRPC endpoint exposing Observation API. This is a temporary solution and Observation Node is expected to take over this responsibility.
