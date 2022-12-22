# Matching Engine and Sealing Engine

### Matching Engine

- Matching engine ingests the Execution Receipts (ERs) from execution nodes and incorporated blocks, validates the ERs and stores then in its `ExecutionTree`.
- Matching core is fully concurrent.
- If receipts are received before the block, we will drop the receipt.
- If receipts are received after the block, but the previous result is missing, we will cache the receipt in the mempool
- If previous result exists, we can validate the receipt, and store it in the execution tree mempool and storage.
- After processing a receipt, we try to find child receipts from the cache, and process them.
- When constructing a new block, the block builder reads receipts and results from the execution tree and add them to the new block’s payload
- Processing finalized block is to prune pending receipts and execution tree mempool.
- Matching core also contains logic for fetching missing receipts (method `requestPendingReceipts`)

  Caution: this logic is not yet fully BFT compliant. The logic only requests receipts for blocks, where it has less than 2 consistent receipts. This is a temporary simplification and incompatible with the mature BFT protocol! There might be multiple consistent receipts that commit to a wrong result. To guarantee sealing liveness, we need to fetch receipts from *all*  ENs, whose receipts we don't have yet. There might be only a single honest EN and its result might occasionally get lost during transmission.

- **Crash recovery:**
    - Crash recovery is implemented in the consensus block builder (`module/builder/consensus/builder.go` method `repopulateExecutionTree`).
    - During normal operations, we query the tree for "all receipts, whose results are derived from the latest sealed and finalized result". This requires the execution tree to know what the latest sealed and finalized result is.

      The builder adds the result for the latest block that is both sealed and finalized, without any Execution Receipts. This is sufficient to create a vertex in the tree. Thereby, we can traverse the tree, starting from the sealed and finalized result, to find derived results and their respective receipts.

    - In a second step, the block builder repopulates the builder adds all known receipts for any finalized but unsealed block to the execution tree.
    - Lastly, the builder adds receipts from any valid block that descends from the latest finalized block.
    - Note:

      It is *strictly required* to add the result for the latest block that is both sealed and finalized. This is necessary because the ID of the latest result is given as a query to find all descending results during block construction. Without the sealed result, the block builder could not proceed.

      In contrast, conceptually it is *not strictly required* for the builder to also add the receipts from the known blocks (finalized unsealed blocks and all pending blocks descending from the latest finalized block). This is because this matching engine *would* re-request these (method `requestPendingReceipts`) by itself. Nevertheless, relying on re-fetching already known receipts would add a large latency overhead after recovering from a crash.


### Sealing Engine

- The sealing engine ingests:
    - Result approvals from the Verification nodes (message `ResultApproval`)
    - It also re-requests missing result approvals and ingests the corresponding responses (`ApprovalResponse`)
    - Execution Results from incorporated blocks. Sealing engine can trust those incorporated results are structurally valid, because block incorporating the result has passed the node's internal compliance engine.

      Caution: to not compromise sealing liveness, the sealing engine cannot drop any incorporated blocks or any results contained in it. Otherwise, it cannot successfully create a seal for the incorporated results.

- Sealing Engine will forward receipts and approvals to the sealing core. Receipts and approvals are concurrently processed by sealing core.
- The purpose for the Sealing Core is to
    1. compute verifier assignments for the incorporated results
    2. track the approvals that match the assignments
    3. generate a seal once enough approvals have been collected
- For sealing core to concurrently process receipts, it maintains an `AssignmentCollectorTree`, which is a fork-aware structure holding the assignments + approvals for each incorporated result. Therefore, it can ingest results and approvals in arbitrary order.
    - Each vertex (`AssignmentCollector`) in the tree is responsible for managing approvals for one particular result.
    - A result might be incorporated in multiple blocks on different block forks. 
    - Results incorporated on different forks will lead to different assignment for verification nodes. For example, consider block `B` with two children `C` and `D`:
      ```
            ┌ C   
         B <   
            └ D
      ```
      `C` and `D` can each incorporate the same result `r` for block `B`, but the verifier assignment would be different. Hence, there is one `AssignmentCollector` for `r` stored in the `AssignmentCollectorTree`.
     
      Although the approvals are the same for results incorporated in different forks, a valid approval for a certain incorporated result on one fork usually cannot be used for the same result incorporated in a different fork.
      This is because verifiers are assigned to check individual chunks of a result and the assignments for a particular chunk differ with high probability from fork to fork. To guarantee correctness, approvals are only accepted
      from verifiers that are assigned to check exactly this chunk. Hence, one `AssignmentCollector` can hold multiple `ApprovalCollector`s. An `ApprovalCollector` manages one particular verifier assignment, i.e. it corresponds to one particular _incorporated_ result.
      So in our example above, we would have one `ApprovalCollector` for result `r` being incorporated in block `C` and another `ApprovalCollector` for result `r` being incorporated in block `D`.
    - As forks are reasonably common and therefore multiple assignments for the same chunk, we ingest result approvals high concurrency, as follows:
      - `AssignmentCollectorTree` has an internal `RWMutex`, which most of the time is accessed in read-mode. Specifically, adding a new approval to an `AssignmentCollector` that 
        already exists only requires a read-lock on the `AssignmentCollectorTree`. 
      - The sealing engine listens to `OnBlockIncorporated` events. Whenever a new execution result is found in the block, sealing core mutates the `AssignmentCollectorTree` state
        by adding a new `AssignmentCollector` to the tree. Encountering a previously unknown result and pruning operations (details below) are the only times, when the `AssignmentCollectorTree` acquires a write lock.   
      - While not very common, the same approval might be usable for multiple assignments. Therefore, we verify the approval only once and if valid, 
        we forward it to the respective `ApprovalCollector`. If any assignment has collected enough approval, we will generate a seal. Note that the seal (same as the assignment) can only be used on one fork.
        If a consensus leader is building a new block on a fork, it will only add new seals for that fork to the new block.
      - Also `ApprovalCollector`s can ingest approvals concurrently. Verifying the approvals is done without requiring a lock. A write-lock on the `ApprovalCollector` is only held while adding the approval to an internal map,
        which is extremely fast and minimizes any lock contention.
- In order to limit the memory usage of the `AssignmentCollectorTree`, we need to prune it by sealed height. Therefore, we subscribe to `OnFinalizedBlock` events and use the sealed height to prune the tree.
  When pruning the tree, we keep the node at the same height as the last sealed height, so that we know which fork actually connects to the sealed block.
- When receiving an approval before receiving the corresponding result, we haven’t created the corresponding assignment collector. Therefore, we cache the approvals in an `approvalCache` within the sealing core.  Note: this is not strictly necessary, because the approval could be re-requested by the sealing engine (method `requestPendingApprovals`). Nevertheless, we found that caching the approvals is an important performance optimization to reduce sealing latency.

  Race condition:
    - Since incorporated results and approvals are processed concurrently, when a worker is processing an approval and learned the result is missing, this state might be stale, because another working could be adding the result at this moment.
    - A naive implementation could first check whether a corresponding result is present and otherwise just add the approval to the cache. The other worker concurrently adding the result and checking the cache for corresponding approvals might not see the approvals to that are concurrently added to the `approvalCache.` In this case, some approvals might be dropped into the cache but not added to the `AssignmentCollector`.

      This is acceptable (though not ideal) because we will be re-requesting those approvals again. Therefore, there is no sealing liveness issue, but potentially a performance penalty.

    - In order to solve this, we let approval worker double check again after adding the result to the cache, whether the `AssignmentCollector` is now present. In this edge case, we move the approval from the cache and re-process it.
- The `AssignmentCollectorTree` tracks whether the result is derived from the latest result with a finalized seal. If interim results are unknown (due to concurrent ingestion of incorporated results), approvals are only cached (status `CachingApprovals`). Once all interim results are received, the `AssignmentCollectorTree` changes the status of the result and all its *connected* children to be processable (status `VerifyingApprovals`).

  In addition, there are two different scenarios, where we want to stop processing any approvals:
  1. Blocks are orphaned, if they are on forks other than the finalized fork. For orphaned blocks, we don’t need to process approvals. However, even if we generated seals for those orphaned blocks, the builder will ensure we won’t add them to new blocks, as the new block always extends the finalized fork.
  2. It is possible that there are conflicting execution results for the same block, in which case only one results will eventually have a _finalized_ sealed and all conflicting results are orphaned.
     Also in this case, the processing of all approvals for the conflicting result and any of its derived results is stopped.
  
  In both cases, we label the execution result as `Orphaned`. Conceptually, these different processing modes of  `CachingApprovals` vs. `VerifyingApprovals` vs. `Orphaned` pertain
  to an execution result. Therefore, they are implemented in the `AssignmentCollector`. Algorithmically, the `AssignmentCollector` interface is implemented by a state machine, aka `AssignmentCollectorStateMachine`, which allows the following state transitions:
    - `CachingApprovals` -> `VerifyingApprovals`
    - `VerifyingApprovals` -> `Orphaned`
    - `CachingApprovals` -> `VerifyingApprovals`
  The logic for each of the three states is implemented by dedicated structs, which the `AssignmentCollectorStateMachine` references through an atomic variable:
    - 1. `CachingAssignmentCollector` implements the state `CachingApprovals`: the execution result has been received, but the previous result has not been received (or the previous result does not connect to a sealed result). We cache the approvals without validating them.
    - 2. `VerifyingAssignmentCollector` implements the state `VerifyingApprovals`: both the execution result and all previous results are known. We validate approvals, and aggregate them to a seal once the necessary number of approvals have been collected.  
    - 3. `OrphanAssignmentCollector` implements the state `Orphaned`: the executed block is conflicting with finalized blocks, or the result conflicts with another result that has a finalized seal. We drop all already ingested approvals and any future approvals.
- While one worker is processing a result approval (e.g. caching it in the `CachingAssignmentCollector`), a different worker might concurrently trigger a state transitions. Therefore, we have to be careful that workers don't execute their action on a stale state.
  Therefore, we double-check if the state was changed _after_ the operation and if the state changed (relatively rarely), we redo the operation:
    - If the worker found the state was `CachingApprovals`, after caching the approval, and found the state becomes `VerifyingApprovals` or `Orphaned`, it needs to take the approval out of the cache and reprocess it.
    - If the worker found the state was `VerifyingApprovals`, and after verified the approval, and aggregated, and found the state becomes `Orphaned`, no further action is needed. 
- **Crash recovery:**

  The sealing core has the method `RepopulateAssignmentCollectorTree` which restores the latest state of the `AssignmentCollectorTree` based on local chain state information. Repopulating is split into two parts:

  1. traverse forward all finalized blocks starting from last sealed block till we reach last finalized block . (lastSealedHeight, lastFinalizedHeight]
  2. traverse forward all unfinalized(pending) blocks starting from last finalized block.

  For each block that is being traversed, we collect the incorporated execution results and process them using `sealing.Core`
