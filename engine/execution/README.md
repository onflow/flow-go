# Execution node overview

### Terminology

- Computation - is a process of evaluating block content and yielding list of changes.  
- Execution - complete, Flow protocol compliant operation of executing block. It manages environment for computation of block, stores execution states, creates execution receipt among others.

## Architecture overview

### Ingestion engine
Central component - input of the node - execution state owner, only component allowed to mutate it. 
It receives blocks, assembles them (gather collections) and forms execution queue (fork-aware ordering).
Once a block is assembled and ready to be computed (full content of transaction is known and valid, previous execution state is available) it sends `ExecutableBlock` for computation (to Computation engine)
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
 
 ### Operation
 Upon receiving block requests for it's collections are sent.
 Upon receiving collection, they are put into appropriate blocks.
 If after those operation blocks become executable (all collections present - or empty block, plus known execution state) - it is executed in separate Go routine.
 After execution is finished, it passes newly created execution state to its children, and if they are now ready - they are, repeating the loop.
 
