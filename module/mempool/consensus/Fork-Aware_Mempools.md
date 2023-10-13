## Determining includability of Execution Receipt:

### Problem Description 
A consensus primary knows a set `S` of Execution Receipts, some of which might not be known to other consensus replicas.
The primaries Fork-Choice rule decides to build on top of block `F`. When constructing the payload, the primary must 
decide which ExecutionReceipts to incorporated in the payload.
* Consider the fork `<- A <- B <- ... <- E <- F` leading up to block `F`. Here, `A` denotes the latest sealed block in that fork.
* For incorporating Execution Receipts, we can restrict our consideration to the section `B <- ... <- E <- F` of the fork, 
  as all earlier blocks have already been sealed as of `F`. 

_Notation_ 

We use the following notation
* `r[B]` is an execution result for block `B`. If there are multiple different results for block `B`, we add an index, e.g. `r[B]_1`, `r[B]_2`, ...
* `Er[r]` is a execution receipt vouching for result `r`. For example `Er[r[C]_2]` is the receipt for result `r[C]_2`
* an Execution Receipt `r` has the following fields:
  * `PreviousResultID` denotes the result `ID` for the parent block that has been used as starting state for computing the current block

![Execution Tree](/docs/ExecutionResultTrees.png)

### Criteria for Incorporating Execution Receipts

Let Er<sup>(1)</sup>, Er<sup>(2)</sup>, ..., Er<sup>(K)</sup> be the receipts included in the _child_ of block `F`. 

There are multiple criteria that have to be satisfied for a receipt to be incorporated in the payload:
1. Receipts must be for unsealed blocks on the fork that is being extended. Formally:
   * Er<sup>(i)</sup> must be for one of the blocks `B, ..., F`
2. No duplication of incorporated receipts. Formally:
   * There are no duplicates in Er<sup>(1)</sup>, Er<sup>(2)</sup>, ..., Er<sup>(K)</sup>
   * _And_ for each `Er` ∈ {Er<sup>(1)</sup>, Er<sup>(2)</sup>, ..., Er<sup>(K)</sup>}: 
     
     `Er` was _not_ incorporated in any of the blocks `B, ..., F`
3. The parent result (`PreviousResultID`) must have been previously incorporated (either in ancestor blocks or earlier in the new block itself). Formally: 
   * For each `Er` ∈ {Er<sup>(1)</sup>, Er<sup>(2)</sup>, ..., Er<sup>(K)</sup>}:
     * `Er.PreviousResultID` is the sealed result
     * _or_ there exists an Execution Receipt `Er'` that was incorporated in the blocks `B, ..., F`
       with `Er'.ExecutionResult.ID() == Er.PreviousResultID`
     * _or_ there exists an Execution Receipt in the list Er<sup>(1)</sup>, Er<sup>(2)</sup>, ..., Er<sup>(K)</sup> _prior_ to `Er`
       with `Er'.ExecutionResult.ID() == Er.PreviousResultID`
  

Note that the condition cannot be relaxed to: "there must be any ExecutionResult for the parent block be included in the fork" . It must be specifically the parent result referenced by PreviousResultID.

### Problem formalization

As illustrated by the figure above, the ExecutionResults form a tree, with the last sealed result as the root. 
* All Execution Receipts committing to the same result from an [equivalence class](https://en.wikipedia.org/wiki/Equivalence_class) and can be 
represented as one vertex in the [Execution Tree](/docs/ExecutionResultTrees.png).
* Consider the results `r[A]` and `r[B]`. As `r[A]`'s output state is used as the staring state to compute block `B`, 
  we can say: "from result `r[A]` `computation` (denoted by symbol `Σ`) leads to `r[B]`". Formally:     
  ```
    r[A] Σ r[B]
  ```
  Here, `Σ` is a [binary relation](https://en.wikipedia.org/wiki/Binary_relation) (more specifically a homogeneous binary relation). 
  Furthermore, consider the case:
   * `r[A] Σ r[B]` (i.e. from result `r[A]` `computation` leads to `r[B]`) 
   * `r[B] Σ r[C]_1` (i.e. from result `r[B]` `computation` leads to `r[C]_1`)
  
  then we can summarize that from result `r[A]` `computation` leads to `r[C]_1`. Formally:
  ```
    from r[A] Σ r[B] and r[B] Σ r[C]_1 it follows that r[A] Σ r[C]_1 
  ```
  Hence,  `Σ` is a [transitive relation](https://en.wikipedia.org/wiki/Binary_relation).

Note:
* `computation` (`Σ`) does _not_ restricted to honest computation. Rather, it means computation proclaimed by an execution node (and backed by its stake).  

### Algorithmic solution

Lets break up the problem into 3 steps:
1. For the first step, lets ignore the receipts already included in the fork. Lets start with only the `sealed_state` and ask:
   ```
   What is the largest set of Execution Receipts that are potential candidates for inclusion in the block I am building?
   ```
   This is necessarily a super-set of the receipts already included in the fork, as a correct solution should at least reproduce those Receipts 
   and potentially others, which haven't been included.
2. We need a suitable ordering so that any receipt's parent result is listed before. 
3. From the result of Step 2, we can then remove the Receipts already included in the fork.

By construction, this generates a _correct and complete_ solution satisfying the above-listed Criteria for Incorporating Execution Receipts.  

#### Step 1: largest set of Execution Receipts that are potential candidates for inclusion

From the perspective of the primary, all Execution Receipts whose results `r` satisfy `sealed_state Σ r` are candidates for inclusion in the block. 
Formally, **the transitive closure of the binary relation `Σ` on the `sealed_state` yields are candidates for inclusion in the block.**

[Wikipedia](https://en.wikipedia.org/wiki/Reachability): For a directed graph `G = (V , E)`, with vertex set `V` and edge set `E`,
the [reachability relation](https://en.wikipedia.org/wiki/Reachability) of `G` is the transitive closure of `E`.   
[Reachability](https://en.wikipedia.org/wiki/Reachability) refers to the ability to get from one vertex to another within a graph. 
A vertex `s` can reach a vertex `t` if there exists a sequence of adjacent vertices (i.e. a path) which starts with `s`
and ends with `t`.

_Available algorithms:_ 
There are a variety of algorithms for computing the transitive closure / reachability in a directed graph with different runtime complexities (e.g. 
[Floyd–Warshall algorithm](https://en.wikipedia.org/wiki/Floyd%E2%80%93Warshall_algorithm), DFS etc). 
A good overview is given in the [Panagiotis Parchas's lecture notes](http://www.cse.ust.hk/~dimitris/6311/L24-RI-Parchas.pdf) 
and [Transitive Closure of a Graph](https://www.techiedelight.com/transitive-closure-graph/). 
The trade-offs are mainly between upfront Construction time vs Query time. 

For our specific problem, we assume that the graph frequently changes due to new results being published. 
Furthermore, we know that our graph is a Tree and hence sparse. Therefore, **running depth-first search (DFS) from the `sealed_state`
(or any other tree search algorithm) has optimal runtime complexity** of `O(|V|+|E|)`.  

#### Step 2: suitable ordering

DFS already lists Execution Results in the desired order.  

#### Step 3: remove the Receipts already included in ancestors 

We can simply store the Receipts that are already included in the fork in a lookup table `M`.
When searching the tree in step 1, we skip all receipts that are in `M` on the fly. 


## Further reading
* [Lecture notes on directed Graphs](http://web.archive.org/web/20180219025720/https://orcca.on.ca/~yxie/courses/cs2210b-2011/htmls/notes/16-directedgraph.pdf)
* [Graph Algorithms and Network Flows](https://hochbaum.ieor.berkeley.edu/files/ieor266-2014.pdf)
* Paper: [The Serial Transitive Closure Problem for Trees](https://www.math.ucsd.edu/~sbuss/ResearchWeb/transclosure/paper.pdf)






