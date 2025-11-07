Asynchronous Collection Indexing — Design

1) Purpose & Outcomes

Goal. Index block collections reliably without overloading the node, even when finalized blocks arrive faster than we can index.

Outcomes.

Workers (job processors) stay focused on a small, sliding window of heights just above the latest indexed height.

Collection retrieval and indexing are decoupled from finalization and from each other.

Execution Data Indexer (EDI) provides the preferred source for collections; collection requests are only sent when EDI falls behind a configurable threshold.

2) High-Level Flow

Finalization happens → a lazy signal wakes the Job Consumer.

Job Consumer consults: (a) Progress Tracker → highest indexed height; (b) Jobs module → latest safe (head) height. It computes a bounded work window [indexed+1 .. min(indexed+N, head)].

For each height in the window, the consumer spins up (or reuses) a Job Processor.

The Job Processor checks if the block is already indexed. If yes → finish immediately. If not:

It enqueues the block’s missing collection IDs into MissingCollectionQueue (MCQ) with a completion callback.

It checks how far EDI lags behind the block height. If the lag exceeds a configured threshold, it triggers CollectionRequester to fetch collections; otherwise, it waits for EDI to deliver them.

As collections arrive (from requester or EDI), the Job Processor forwards them to MCQ. When MCQ detects a block is now complete, the processor passes the collections to BlockCollectionIndexer to store+index, then calls MCQ to mark the job done.

Jobs may complete out of order; the progress tracker advances once any gaps below are closed.

3) Core Interfaces

BlockCollectionIndexer

Stores and indexes collections for a given block height; provides the latest indexed height for windowing and fast no-op checks.

type BlockCollectionIndexer interface {
    LatestIndexedHeight() uint64

    // If blockHeight <= LatestIndexedHeight(), return quickly.
    // Otherwise: lock, re-check, then persist+index collections.
    // Double-check pattern minimizes lock contention.
    OnReceivedCollectionsForBlock(blockHeight uint64, cols []*flow.Collection) error
}

MissingCollectionQueue (MCQ)

In-memory coordinator for jobs and callbacks. MCQ does not index; it only tracks missing collections per height and announces when a height becomes complete.

type MissingCollectionQueue interface {
    EnqueueMissingCollections(blockHeight uint64, ids []flow.Identifier, callback func()) error
    OnIndexedForBlock(blockHeight uint64)
	// On receipt of a collection, MCQ updates internal state and, if a block
	// just became complete, returns: (collections, height, true).
	// Otherwise, returns (nil, 0, false).
	OnReceivedCollection(collectionID flow.Identifier) ([]*flow.Collection, uint64, bool)
}

CollectionRequester

Abstracts the engine that requests collections by ID (e.g., from collection nodes).

type CollectionRequester interface {
    RequestCollections(ids []flow.Identifier) error
}

JobProcessor

Owns the state of ongoing jobs (delegated to MCQ) and orchestrates request → receive → index → complete.

type JobProcessor interface {
    ProcessJob(ctx irrecoverable.SignalerContext, job module.Job, done func()) error
    OnReceivedCollectionsForBlock(blockHeight uint64, cols []*flow.Collection) error
}

4) Job Consumer (Windowed Throttling)

Why: Prevent node overload when finalized heights advance rapidly.

How:

Reads latestIndexed = BlockCollectionIndexer.LatestIndexedHeight().

Reads head = Jobs.Head() (latest height safe to work on).

Defines window size K. Range: [latestIndexed+1 .. min(latestIndexed+K, head)].

Assigns one JobProcessor per height in this range.

Lazy notification: Finalization pushes a single, coalescing signal to workSignal (buffer size 1). The consumer wakes, recomputes the range, and may ignore new jobs if already at capacity.

5) Job Lifecycle

Spawn/Assign. Consumer gives (height, job, doneCb) to a JobProcessor.

Already Indexed? Processor queries storage. If yes → doneCb() and return.

Track Missing. MCQ.EnqueueMissingCollections(height, collectionIDs, doneCb).

Check EDI Lag. Compare height with ediIndexedHeight(). If lag ≤ threshold, wait for EDI; if lag > threshold, trigger CollectionRequester.RequestCollections(ids).

Receive Collections.

Processor calls MCQ.OnReceivedCollection(id).

If complete, processor calls BlockCollectionIndexer.OnReceivedCollectionsForBlock(height, cols) and then MCQ.OnIndexedForBlock(height).

Progress Advancement. Out-of-order completion allowed; progress advances only when lower gaps close.

Crash/Restart. On restart, re-created jobs short-circuit if already indexed.

6) Execution Data Indexer (EDI) Integration

EDI serves as the primary source of collections. The system dynamically decides whether to fetch collections based on EDI’s progress.

Lag-based hybrid logic:

Track ediIndexedHeight() — the latest height for which EDI has collections.

Define a lag threshold, EDILagThreshold, in number of blocks.

For block height h:

If (h - ediIndexedHeight()) <= EDILagThreshold: rely on EDI; no fetching.

If (h - ediIndexedHeight()) > EDILagThreshold: trigger the collection fetcher to request collections from nodes.

Behavior summary:

If EDI is up to date or within threshold → no fetches.

If EDI is behind beyond threshold → start fetching.

Setting a very large threshold effectively mimics the previous EDI-only mode.

Why EDI goes through JobProcessor: To keep job state consistent—update MCQ, suppress redundant fetches, and advance the queue.

Contention handling:

If EDI lags → fetching fills the gap.

If EDI leads → consumer advances; requester stops fetching.

If close → minimal contention; indexing is faster than receipt.
