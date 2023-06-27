## Block Consumer ([consumer.go](verification%2Fassigner%2Fblockconsumer%2Fconsumer.go))
The `blockconsumer` package efficiently manages the processing of finalized blocks in Verification Node of Flow blockchain.
Specifically, it listens for notifications from the `Follower` engine regarding finalized blocks, and systematically 
queues these blocks for processing. The package employs parallel workers, each an instance of the `Assigner` engine, 
to fetch and process blocks from the queue. The `BlockConsumer` diligently coordinates this process by only assigning 
a new block to a worker once it has completed processing its current block and signaled its availability. 
This ensures that the processing is not only methodical but also resilient to any node crashes. 
In case of a crash, the `BlockConsumer` resumes from where it left off, reassigning blocks from the queue to workers,
thereby guaranteeing no loss of data.

## Assigner Engine 
The `Assigner` [engine](verification%2Fassigner%2Fengine.go) is an integral part of the verification process in Flow, 
focusing on processing the execution results in the finalized blocks, performing chunk assignments on the results, and
queuing the assigned chunks for further processing. The Assigner engine is a worker of the `BlockConsumer` engine,
which assigns finalized blocks to the Assigner engine for processing.
This engine reads execution receipts included in each finalized block, 
determines which chunks are assigned to the node for verification, 
and stores the assigned chunks into the chunks queue for further processing (by the `Fetcher` engine).

The core behavior of the Assigner engine is implemented in the `ProcessFinalizedBlock` function. 
This function initiates the process of execution receipt indexing, chunk assignment, and processing the assigned chunks. 
For every receipt in the block, the engine determines chunk assignments using the verifiable chunk assignment algorithm of Flow.
Each assigned chunk is then processed by the `processChunk` method. This method is responsible for storing a chunk locator in the chunks queue, 
which is a crucial step for further processing of the chunks by the fetcher engine. 
Deduplication of chunk locators is handled by the chunks queue.
The Assigner engine provides robustness by handling the situation where a node is not authorized at a specific block ID. 
It verifies the role of the result executor, checks if the node has been ejected, and assesses the node's staked weight before granting authorization.
Lastly, once the Assigner engine has completed processing the receipts in a block, it sends a notification to the block consumer. This is inline with 
Assigner engine as a worker of the block consumer informing the consumer that it is ready to process the next block.
This ensures a smooth and efficient flow of data in the system, promoting consistency across different parts of the Flow architecture.

### Chunk Locator
A chunk locator in the Flow blockchain is an internal structure of the Verification Nodes that points to a specific chunk 
within a specific execution result of a block. It's an important part of the verification process in the Flow network,
allowing verification nodes to efficiently identify, retrieve, and verify individual chunks of computation.

```go
type ChunkLocator struct {
    ResultID flow.Identifier // The identifier of the ExecutionResult
    Index    uint64         // Index of the chunk
}
```
- `ResultID`: This is the identifier of the execution result that the chunk is a part of. The execution result contains a list of chunks which each represent a portion of the computation carried out by execution nodes. Each execution result is linked to a specific block in the blockchain.
- `Index`: This is the index of the chunk within the execution result's list of chunks. It's an easy way to refer to a specific chunk within a specific execution result.

**Note-1**: The `ChunkLocator` doesn't contain the chunk itself but points to where the chunk can be found. In the context of the `Assigner` engine, the `ChunkLocator` is stored in a queue after chunk assignment is done, so the `Fetcher` engine can later retrieve the chunk for verification.
**Note-2**: The `ChunkLocator` is never meant to be sent over the networking layer to another Flow node. It's an internal structure of the verification nodes, and it's only used for internal communication between the `Assigner` and `Fetcher` engines.


## ChunkConsumer 
The `ChunkConsumer` ([consumer](verification%2Ffetcher%2Fchunkconsumer%2Fconsumer.go)) package orchestrates the processing of chunks in the Verification Node of the Flow blockchain. 
Specifically, it keeps tabs on chunks that are assigned for processing by the `Assigner` engine and systematically enqueues these chunks for further handling. 
To expedite the processing, the package deploys parallel workers, with each worker being an instance of the `Fetcher` engine, which retrieves and processes the chunks from the queue. 
The `ChunkConsumer` administers this process by ensuring that a new chunk is assigned to a worker only after it has finalized processing its current chunk and signaled that it is ready for more. 
This systematic approach guarantees not only efficiency but also robustness against any node failures. In an event where a node crashes,
the `ChunkConsumer` picks up right where it left, redistributing chunks from the queue to the workers, ensuring that there is no loss of data or progress.

## Fetcher Engine


# Notifier
The Notifier implements the following state machine
![Notifier State Machine](/docs/NotifierStateMachine.png)

The intended usage pattern is:
* there are goroutines, aka `Producer`s, that append work to a queue `pendingWorkQueue`
* there is a number of goroutines, aka `Consumer`s, that pull work from the `pendingWorkQueue`
   * they consume work until they have drained the `pendingWorkQueue`
   * when they find that the `pendingWorkQueue` contains no more work, they go back to 
     the notifier and await notification 

![Notifier Usage Pattern](/docs/NotifierUsagePattern.png)

Note that the consumer / producer interact in a _different_ order with the `pendingWorkQueue` vs the `notifier`:
* the producer first drops its work into the queue and subsequently sends the notification 
* the consumer first processes elements from the queue and subsequently checks for a notification 
Thereby, it is guaranteed that at least one consumer routine will be notified when work is added

