# Sync Engine (Core Protocol)

| Status        | Proposed                                                  |
:-------------- |:--------------------------------------------------------- |
| **FLIP #**    | 1697                                                      |
| **Author(s)** | Simon Zhu (simon.zhu@dapperlabs.com)                      |
| **Sponsor**   | Simon Zhu (simon.zhu@dapperlabs.com)                      |
| **Updated**   | 11/29/2021                                                |

## Objective

Redesign the synchronization protocol to improve efficiency, robustness, and Byzantine fault tolerance.

## Current Implementation

The current synchronization protocol implementation consists of two main pieces:
* The [Sync Engine](https://github.com/onflow/flow-go/blob/39c455da40c8f0aa6f9962c48f4cd34a5cbacfc0/engine/common/synchronization/engine.go) interfaces with the network layer and handles sending synchronization requests to other nodes and processing responses.
* The [Sync Core](https://github.com/onflow/flow-go/blob/39c455da40c8f0aa6f9962c48f4cd34a5cbacfc0/module/synchronization/core.go) implements the core logic, configuration, and state management of the synchronization protocol.

There are three types of synchronization requests:
* A [Sync Height Request](https://github.com/onflow/flow-go/blob/39c455da40c8f0aa6f9962c48f4cd34a5cbacfc0/model/messages/synchronization.go#L8-L14) is sent to share the local finalized height while requesting the same information from the recipient. It is replied to with a [Sync Height Response](https://github.com/onflow/flow-go/blob/39c455da40c8f0aa6f9962c48f4cd34a5cbacfc0/model/messages/synchronization.go#L16-L22).
* A [Batch Request](https://github.com/onflow/flow-go/blob/39c455da40c8f0aa6f9962c48f4cd34a5cbacfc0/model/messages/synchronization.go#L34-L40) requests a list of blocks by ID. It is replied to with a [Block Response](https://github.com/onflow/flow-go/blob/39c455da40c8f0aa6f9962c48f4cd34a5cbacfc0/model/messages/synchronization.go#L42-L48).
* A [Range Request](https://github.com/onflow/flow-go/blob/39c455da40c8f0aa6f9962c48f4cd34a5cbacfc0/model/messages/synchronization.go#L24-L32) requests a range of finalized blocks by height. It is replied to with a [Block Response](https://github.com/onflow/flow-go/blob/39c455da40c8f0aa6f9962c48f4cd34a5cbacfc0/model/messages/synchronization.go#L42-L48).

The Sync Core uses two data structures to track the statuses of requestable items:
* [`Heights`](https://github.com/onflow/flow-go/blob/39c455da40c8f0aa6f9962c48f4cd34a5cbacfc0/module/synchronization/core.go#L53) tracks the set of requestable finalized block heights. It is used to generate Ranges for the Sync Engine to request.
* [`BlockIDs`](https://github.com/onflow/flow-go/blob/39c455da40c8f0aa6f9962c48f4cd34a5cbacfc0/module/synchronization/core.go#L54) tracks the set of requestable block IDs. It is used to generate Batches for the Sync Engine to request.

The Sync Engine periodically picks a small number of random nodes to send Sync Height Requests to. It also periodically calls [`ScanPending`](https://github.com/onflow/flow-go/blob/39c455da40c8f0aa6f9962c48f4cd34a5cbacfc0/module/synchronization/core.go#L148-L166) to get a list of requestable Ranges and Batches from the Sync Core, and picks some random nodes to send those requests to.

Each time the Compliance Engine processes a new block proposal, it finds the first ancestor which has not yet been received (if one exists) and calls [`RequestBlock`](https://github.com/onflow/flow-go/blob/39c455da40c8f0aa6f9962c48f4cd34a5cbacfc0/module/synchronization/core.go#L114-L126) to request the missing block ID. `RequestBlock` updates `BlockIDs` by queueing the block ID.

Each time the Sync Engine receives a Sync Height Response, it calls [`HandleHeight`](https://github.com/onflow/flow-go/blob/39c455da40c8f0aa6f9962c48f4cd34a5cbacfc0/module/synchronization/core.go#L95-L112) to pass the received height to the Sync Core, which updates `Heights` by queueing all heights between the local finalized height and the received height.

Each time the Sync Engine receives a Block Response, it calls [`HandleBlock`](https://github.com/onflow/flow-go/blob/39c455da40c8f0aa6f9962c48f4cd34a5cbacfc0/module/synchronization/core.go#L67-L93) to pass each of the received blocks to the Sync Core, which updates the tracked statuses in `Heights` and `BlockIDs`.

### Potential Problems 

* Items in `BlockIDs` do not contain the block height, which means that they cannot be pruned until the corresponding block has actually been received. If a malicious block proposal causes a non-existent parent ID to be queued by the Compliance Engine, the item will not be pruned until the maximum number of attempts is reached.
* After a block corresponding to an item in `BlockIDs` is received, the item is not pruned until the local finalized height surpasses the height of the block. If a node is very far behind, `BlockIDs` could grow very large before the local finalization catches up.
* When the Sync Engine calls `ScanPending`, it passes in the local finalized height, which the Sync Core uses to prune requestable items. Since `Heights` and `BlockIDs` are both implemented using Go maps, pruning them involves iterating through all items to find the ones for which the associated block height is lower than the local finalized height, which is inefficient. Furthermore, pruning is triggered on every call to `ScanPending`, even if the local finalized height has not changed.
* The implementation of `ScanPending` is split into three steps:
    * Iterate through `Heights` and `BlockIDs` and [find all requestable heights and block IDs](https://github.com/onflow/flow-go/blob/39c455da40c8f0aa6f9962c48f4cd34a5cbacfc0/module/synchronization/core.go#L264-L326).
    * Group these requestable items into [Ranges](https://github.com/onflow/flow-go/blob/39c455da40c8f0aa6f9962c48f4cd34a5cbacfc0/module/synchronization/core.go#L360-L415) and [Batches](https://github.com/onflow/flow-go/blob/39c455da40c8f0aa6f9962c48f4cd34a5cbacfc0/module/synchronization/core.go#L417-L439). 
    * [Select a subset of these Ranges and Batches to return](https://github.com/onflow/flow-go/blob/39c455da40c8f0aa6f9962c48f4cd34a5cbacfc0/module/synchronization/core.go#L441-L454) based on a configurable limit on the maximum number of in-flight requests. 

  While conceptually easy to understand, this implementation is inefficient and performs many more loop iterations than necessary.
* `HandleHeight` iterates over the entire range from the local finalized height to the received height, queueing all new heights and requeueing heights which have already been received. This can be expensive if the node is very far behind.
* The Sync Core optimistically sets the status of an item in `Heights` as Received as soon as *any* block with the corresponding height is received, even though it has no way of knowing whether the received block has actually been finalized by consensus. This could cause the height to stop being requested before the finalized block has actually been received. It's also possible that this could cause `Heights` to become fragmented (smaller requestable ranges).
* Processing a Range Request response may or may not advance the local finalized height. There are two reasons why it may not progress:
  * The response contains blocks that are not actually finalized (e.g. received from a malicious node).
  * More blocks are needed to form a Three-Chain and advance the local finalized height. Although this case becomes increasingly unlikely with larger ranges, it is theoretically still possible under the relaxed version of the [HotStuff](https://arxiv.org/abs/1803.05069) algorithm.

  The Sync Core does not account for the second case, and so it is possible that the Sync Engine gets stuck requesting the same range over and over.
* There is no way to determine whether a Sync Height Response is honest or not. If `HandleHeight` is called for every received Sync Height Response, an attacker could cause `Heights` to grow unboundedly large by sending a Sync Height Response with an absurdly high height.

## Proposal

The `RequestBlock` API should be updated to accept a block height, which should be stored with the queued item in `BlockIDs`. This will allow items in `BlockIDs` to be pruned as soon as the local finalized height surpasses their associated block heights. This also allows the Sync Core to ignore calls to `RequestBlock` for block heights which exceed the local finalized height by more than a configurable threshold. This helps to reduce the amount of resources spent tracking and requesting blocks which which cannot immediately be finalized anyways.

Instead of pruning on every call to `ScanPending`, the Sync Core should keep track of the local finalized height from the latest call to `ScanPending`, and only trigger pruning if the height has actually changed. If necessary, it's possible to optimize the performance of pruning and avoid iterating through every item in `BlockIDs` by maintaining an additional mapping from block heights to the set of requestable block IDs at each height. 

Instead of sending synchronization requests via gossip, we should directly create a new stream to another node for each request and validate the response we receive:
* A Range Request response should contain a single chain of blocks which begins at the start height of the requested range and is no longer than the size of the requested range.
* A Batch Request response should contain a subset of the requested block IDs.

This eliminates any ambiguity about whether a response corresponds to a Batch or Range Request, so we can avoid optimistically setting the statuses of heights as Received for responses to Batch Requests.

At any time, there is a single range of heights that the Sync Engine actively requests, which is tracked by the Sync Core. We call this the Active Range. The Active Range is parameterized by two variables `RangeStart` and `RangeEnd`, which effectively replace the `Heights` map from the existing implementation, but it can be broken up and requested by the Sync Engine in multiple segments. `RangeStart` should be greater than the local finalized block height, and `RangeEnd` should be less than or equal to the target finalized block height (more details below). The logic for updating the Active Range can be abstracted with an interface:

```golang
type ActiveRange interface {
    // Update processes a range of blocks received from a Range Request 
    // response and updates the requestable height range.
    Update(headers []flow.Header, originID flow.Identifier)

    // LocalFinalizedHeight is called to notify a change in the local finalized height.
    LocalFinalizedHeight(height uint64)

    // TargetFinalizedHeight is called to notify a change in the target finalized height.
    TargetFinalizedHeight(height uint64)

    // Get returns the range of requestable block heights.
    Get() flow.Range
}
```

There are many ways to implement this interface, but one possible approach is as follows:
* Select values for parameters `DefaultRangeSize` and `MinResponses`
* Let `PendingStart` be the first height greater than `LocalFinalizedHeight` that has been received less than `MinResponses` times
* Let `RangeStart` be equal to `LocalFinalizedHeight + 1`
* Let `RangeEnd` be the smaller of `TargetFinalizedHeight` and `PendingStart + DefaultRangeSize`

The reason we keep track of `PendingStart` is to ensure that `RangeEnd` eventually increases even if the local finalized height doesn't. This is needed to address the last item in [Potential Problems](#potential-problems). 

The target finalized height represents the speculated finalized block height of the overall chain, and should reflect the Sync Height Responses that have been received while accounting for the possibility that some of these responses are malicious. Therefore, the Sync Height Response processing logic should incorporate some sort of expiration / filtering mechanism. The details of this logic can be abstracted with an interface:

```golang
type TargetFinalizedHeight interface {
    // Update processes a height received from a Sync Height Response 
    // and updates the finalized height estimate.
    Update(height uint64, originID flow.Identifier)

    // Get returns the estimated finalized height of the overall chain.
    Get() uint64
}
```

One possible approach is to maintain a sliding window of the most recent Sync Height Responses, and take the median of these values. This implies that the target finalized height will always lag slightly behind the true finalized height, which may or may not be a problem depending on the block finalization rate.


