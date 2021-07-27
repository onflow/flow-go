# Tracking result approvals 

## Context

Among other things, an `ExecutionResult` specifies  
* `BlockID`: the ID of the executed block
* `PreviousResultID`: the ID of the result that was used as starting state to run the computation

This implies the `ExecutionResults` form a tree. We call this the **Execution Tree**. However, nodes do not 
need to retain the entire tree, but can prune levels of lower height. By pruning lower levels, the full 
tree decomposes into a forest of subtrees. 

#### Multiple verifier assignments 

For a single Execution Result, there can be multiple assignments of Verification Nodes to the individual chunks.
This is because:
* Execution results are incorporated in blocks.
* The block that incorporates the result is used to determine the verifier assignment.
* The same result `r` can be incorporated in different forks.
* The different blocks incorporating `r` will have different Sources of Randomness.
  Hence, the verifier assignments differ for the different forks.    

At some point, verifier assignments can be discarded. For example, if the block incorporated the result is orphaned.  


## Assignment Collectors

From a computational perspective, it makes sense to group all assignments for the same result: 
* A `ResultApproval` is for a specific execution result. There is nothing in the approval that ties it to
  a specific assignment. So theoretically, the same approval could count towards multiple different assignments. 
* The dominant computational cost when processing a `ResultApproval` is verifying its correctness (cryptographic signature and SPoCK proof).
  In contrast, determining which assignments the approval can be counted towards is negligible. 
  In other words, tracking more than the minimally necessary assignments adds no noteworthy computational overhead)

Hence, it makes sense to define the  [AssignmentCollector](./assignment_collector.go). 
It processes the `ResultApproval`s for one particular execution result: 
* Specifically, it memorizes all known verifier assignments for the result.
* When it receives a `ResultApproval`, it verifies the approval _once_ and then adds the approval to eligible assignments.


### Assignment Collector Tree
It is beneficial to arrange the Assignment Collectors as a tree (or a forest depending on pruning).
This allows us to draw conclusions 

As illustrated by the figure above, the `AssignmentCollector`s form a tree 
Formally, we define:
* All Verification assignments for the same result from an [equivalence class](https://en.wikipedia.org/wiki/Equivalence_class). 
  The equivalence class represents one vertex in the `AssignmentCollectorTree`. It is implemented as `AssignmentCollector`. 
* The edges are placed exactly like in the [Execution Tree](/module/mempool/consensus/Fork-Aware_Mempools.md ):
   - Let `r[B]` be an execution result for block `B`. We denote the corresponding `AssignmentCollector` as `c[r[B]]`. 
   - Let `r_parent := r[B].PreviousResultID` be the parent result and `c[r_parent]` the corresponding `AssignmentCollector`.
  
  In the Execution Tree, there is an edge `r[B] --> r_parent`. Correspondingly, we place the edge
  `c[r[B]] --> c[r_parent]` in the Assignment Collector Tree.
* Lastly, for an `AssignmentCollector`, we define a `level` as the height of the executed block.  

![Assignment Collector Tree](/docs/AssignmentCollectorTree_1.png)


### Orphaning forks in the Assignment Collector Tree

There are two scenarios when we can orphan entire forks of the Assignment Collector Tree:
1. The executed blocks were orphaned. This happens as a consequence of block finalization by the local HotStuff instance.    
2. If an execution result conflicts with a finalized result seal, the result itself as well as all the derived results are orphaned. 

In addition, we can prune all levels in the Assignment Collector Tree that are _below_ the latest finalized seal.   

When an `AssignmentCollector` is orphaned, we can stop processing the corresponding approvals.
However, for the following reason, we should not right away discard the respective vertices in the Assignment Collector Tree:
* We might later learn about derives results (e.g. results for child blocks or different execution results).
* In order to make the determination whether a new `AssignmentCollector` should be orphaned, we retain the orphaned forks in
  the Assignment Collector Tree. When adding an `AssignmentCollector` that extends an already orphaned fork, it is also orphaned.

Orphaned forks will eventually be pruned by height, when sealing processes beyond the fork's height.   

### The Assignment Collector's States

We already have discussed two different modes an assignment collector could operate in:
* `VerifyingApprovals`: it is processing incoming `ResultApproval`s 
* `Orphaned`: it has been orphaned and discards all `ResultApproval`s

The Assignment Collector Tree is populated in a fully concurrent manner. Hence, when adding an `AssignmentCollector`, it can happen 
that the parent is not yet present (see following figure for illustration). This is the mode:
* `CachingApprovals`: the collector caches all approvals without processing them. This mode is relatively rare and largely occurs as
  an edge-case if the node is behind and catching up (i.e. processing many blocks in a relatively short time). If the node is behind, 
  it is likely that the result is already sealed by the time it caught up. We permit a simple implementation that is to not 
  keep track of the order in which it received the approvals. 

![Assignment Collector Tree](/docs/AssignmentCollectorTree_2.png)

In the future, we will also add further states for 'Extensive Checking Mode':
* `ExtensiveCheckingMode`: the `AssignmentCollector` has not received sufficient approvals and sealing is lagging behind
  (exceeding a certain threshold). In this case, the should trigger extensive checkig mode, i.e. request additional 
  verifiers to check.


# Implementation Comments


### Compare And Repeat Pattern

In multiple places we employ the **`Compare And Repeat Pattern`**. This pattern is applicable when we have:
* an atomically-updatable state value
* some business logic that requires an up-to-date value of the state 

With the `Compare And Repeat Pattern`, we guarantee that the business logic was executed with the most
recent state. 

#### Pattern Details

1. Atomically read the state _before_ the operation. 
2. Execute the operation on the retrieved state. 
3. Atomically read the state _after_ the operation. 
    - If the state changed, we updated a stale state. In this case we go back to step 1.
    - If the state remained unchanged, we updated the most recent state. Hence, we are done.

### Key for caching `ResultApproval`s

In case we don't process a `ResultApproval` right away, we generally cache it.
If done naively, the caching could be exploited by byzantine nodes: they could send lots of approvals for the
same chunk but vary some field in their approvals that is not validated right away (e.g. the SPoCK proof). 
If consensus nodes used the `ID` of the full approval as key for the cache, the malicious approvals would have
all different keys, which would allow a malicious node to flood the cache. 

Instead, we only want to cache _one_ approval per Verification node for each specific chunk. Therefore, we use
`ResultApproval.PartialID()`, which is only computed over the chunk-identifier plus the verifier's node ID.  




