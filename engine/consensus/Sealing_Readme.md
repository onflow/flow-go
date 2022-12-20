# Matching Engine and Sealing Engine

### Matching Engine

- Matching engine ingests the Execution Receipts (ERs) from execution nodes and incorporated blocks, validates the ERs and stores then in its `ExecutionTree`.
- Matching core is fully concurrent
- if receipts are received before the block, we will drop the receipt
- If receipts are received after the block, but the previous result is missing, we will cache the receipt in the mempool
- If previous result exists, we can validate the receipt, and store it in the execution tree mempool and storage.
- After processing a receipt, we try to find child receipts from the cache, and process them.
- When constructing a new block, the block builder reads receipts and results from the execution tree and add them to the new block’s payload
- Processing finalized block is to prune pending receipts and execution tree mempool.
- Matching core also contains logic for fetching missing receipts (method `requestPendingReceipts`)

  Caution: this logic is not yet fully BFT compliant. The logic only requests receipts for blocks, where it has less than 2 consistent receipts. This is a temporary simplification and incompatible with the mature BFT protocol! There might be multiple consistent receipts that commit to a wrong result. To guarantee sealing liveness, we need to fetch receipts from *all*  ENs, whose receipts we don't have yet. There might be only a single honest EN and its result might occasionally get lost during transmission.

- **Crash recovery:**
    - Crash recovery is implemented in the consensus block builder (`module/builder/consensus/builder.go` method `repopulateExecutionTree`)
    - During normal operations, we query the tree for "all receipts, whose results are derived from the latest sealed and finalized result". This requires the execution tree to know what the latest sealed and finalized result is.

      The builder adds the result for the latest block that is both sealed and finalized, without any Execution Receipts. This is sufficient to create a vertex in the tree. Thereby, we can traverse the tree, starting from the sealed and finalized result, to find derived results and their respective receipts.

    - In a second step, the block builder repopulates the builder adds all known receipts for any finalized but unsealed block to the execution tree.
    - Lastly, the builder adds receipts from any valid block that descends from the latest finalized block.
    - Note:

      It is *strictly required* to add the result for the latest block that is both sealed and finalized. This is necessary because the ID of the latest result is given as a query to find all descending results during block construction. Without the sealed result, the block builder could not proceed.

      In contrast, conceptually it is *not strictly required* for the builder to also add the receipts from the known blocks (finalized unsealed blocks and all pending blocks descending from the latest finalized block). This is because this matching engine *would* re-request these (method `requestPendingReceipts`) by itself. Nevertheless, relying on re-feting already known receipts would add a large latency overhead after recovering from a crash.


### Sealing Engine

- The sealing engine ingests:
    - Result approvals from the Verification nodes (message `ResultApproval`)
    - It also re-requests missing result approvals and ingests the corresponding responses (`ApprovalResponse`)
    - Execution Results from incorporated blocks. Sealing engine can trust those incorporated results are valid, because the protocol state has validated them.

      Caution: to not compromise sealing liveness, the sealing engine cannot drop any incorporated blocks or any results contained in it. Otherwise, it cannot successfully create a seal for the incorporated results.

- Sealing Engine will forward each receipts and approval to the sealing core. Receipts and approvals are concurrently processed by sealing core.
- The purpose for the Sealing Core is to
    1. compute verifier assignments for the incorporated results
    2. track the approvals that match the assignments
    3. generate a seal once enough approvals have been collected
- For sealing core to concurrently process receipts, it maintains an `AssignmentCollectorTree`, which is a fork-aware structure holding the assignments + approvals for each incorporated result. Therefore, it can ingest results and approvals in arbitrary order.
    - Each vertex (`AssignmentCollector`) in the tree is responsible for collecting approvals one particular incorporated result. Each collector is able to process approvals concurrently. But accessing the tree requires a lock.
    - A result might be incorporated in multiple blocks on different block forks. Results incorporated on different forks will lead to different assignment for verification nodes. Although the approvals are the same for incorporated results on different fork, a valid approval for a certain incorporated result on a fork might be invalid for another incorporated result in a different fork, because the assignment is different and a verification node is not allowed to verify a result if the result is not assigned to it to verify.
- The `AssignmentCollectorTree` tracks whether the result is derived from the latest result with a finalized seal. If interim results are unknown (due to concurrent ingestion of incorporated results), approvals are only cached (status `CachingApprovals`). Once all interim results are received, the `AssignmentCollectorTree` changes the status of the result and all its *connected* children to be processable (status `VerifyingApprovals`).

  Furthermore, if results are orphaned because they conflict with a result that has a finalized seal, the processing of all approvals for the conflicting result and any of its derived results is stopped  (status `Orphaned`)

- In order to limit the memory usage of the `AssignmentCollectorTree`, we need to prune it by sealed height. Therefore, we subscribe to finalized block event and use the sealed height to prune the tree.

  When pruning the tree, we keep the node at the same height as the last sealed height, so that we know which fork actually connects to the sealed block

- It means adding a new result to the tree requires a lock, and is a write lock. A write lock won't block reads, so that the worker to process approvals won't be blocked.
- When a result has been added to a tree, it also means we've received the executed block, because otherwise the block, which includes the receipt, won't pass the validation
- We create an approval collector for each incorporated result, when we receive an approval.  We verify the approval only once, if valid, we forward it to multiple approval collectors for each result incorporated in different fork. If any fork has collected enough approval, we will generate a seal, note a seal can only be used on one fork. If a consensus leader is building a new block on a fork, it will only add new seals for that fork to the new block.
- When adding a new result to the tree, we use a Write lock on the three to add the `AssignmentCollector`. However, when receiving an `AssignmentCollector` from the tree to add an approval, we use a read lock, so that processing approvals concurrently won’t be blocked by processing receipts.
- When receiving an approval before receiving the corresponding result, we haven’t created the corresponding assignment collector. Therefore, we cache the approvals in an `approvalCache` within the sealing core.  Note: this is not strictly necessary, because the approval could be re-requested by the sealing engine (method `requestPendingApprovals`). Nevertheless, we found that caching the approvals is an important performance optimization to reduce sealing latency.
- Race condition:
    - Since incorporated results and approvals are processed concurrently, when a worker is processing an approval and learned the result is missing, this state might be stale, because another working could be adding the result at this moment.
    - A naive implementation could first check whether a corresponding result is present and otherwise just add the approval to the cache. The other worker concurrently adding the result and checking the cache for corresponding approvals might not see the approvals to that are concurrently added to the `approvalCache.` In this case, some approvals might be dropped into the cache but not added to the `AssignmentCollector`.

      This is acceptable (though not ideal) because we will be re-requesting those approvals again. Therefore, there is no sealing liveness issue, but potentially a performance penalty.

    - In order to solve this, we let approval worker double check again after adding the result to the cache, whether the `AssignmentCollector` is now present. In this edge case, we move the approval from the cache and re-process it.
- A block would be outdated if it’s on a different fork than the finalized fork. For outdated blocks, we don’t need to collect approvals for them, as they will never be sealed. However, even if we generated seals for those outdated blocks, the builder will ensure we won’t add them to new blocks, as the new block always extends the finalized fork.
- We learned a block become outdated when a new block gets finalized, which makes a different block at the same height and its children to be outdated.
- In other words, if a block becomes outdated, all approval collectors for that block can drop existing approvals and future approvals. In order to do that we need to distinguish different state of the approval collectors, in total, there are three different states:
    - 1. Cache, when the execution result has been received, but the previous result has not been received (or the previous result does not connect to a sealed result). We cache the approvals without validating them
    - 2. Verifying, when both the execution result and all previous results are known, we could validate approvals, and aggregate them
    - 3. Outdated, when the executed block is conflicting with finalized blocks, we could drop all approvals and future approvals.
- The only state transactions are:
    - `CachingApprovals` -> `VerifyingApprovals`
    - `VerifyingApprovals` -> `Orphaned`
    - `CachingApprovals` -> `VerifyingApprovals`
- Because of the state transitions, a worker processing an incorporated result or an approval concurrently might get staled state. One way to solve it is to double check if the state was changed after the operation, if changed, redo the operation.
    - If the worker found the state was `CachingApprovals`, after caching the approval, and found the state becomes `VerifyingApprovals` or `Orphaned`, it needs to take the approval out of the cache and reprocess it
    - If the worker found the state was `VerifyingApprovals`, and after verified the approval, and aggregated, and found the state becomes `Orphaned`, no action need to take.
- **Crash recovery:**

  The sealing core has the method `RepopulateAssignmentCollectorTree` which restores the latest state of the `AssignmentCollectorTree` based on local chain state information. Repopulating is split into two parts:

  1. traverse forward all finalized blocks starting from last sealed block till we reach last finalized block . (lastSealedHeight, lastFinalizedHeight]
  2. traverse forward all unfinalized(pending) blocks starting from last finalized block.

  For each block that is being traversed, we collect the incorporated execution results and process them using `sealing.Core`
