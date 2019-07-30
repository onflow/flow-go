# Consensus 

The consensus stream covers all aspects where final decisions are made about the
network's computation state. 

More specifically, collection covers the following:

**Block Production** 
- Receiving collections from Collector Nodes
- Block formation (generating a candidate block and proposing it to the network)
- BFT consensus to agree on the collections included in a block
- Random Beacon which adds entropy for seeding pseudo-random-number generators to the block
- Block publishing: broadcasting the resulting finalized block to the entire network 

**Block Sealing**  
to be determined whether it will live in this stream of in a separate stream


## Terminology

* **Collection** - A set of transactions bundled together by a [Collection Node Cluster](../../../internal/roles/collect)
* **Consensus Nodes (CNs)** - collectively produce finalized blocks (including running the random beacon) 
* **Proto Block** - _Candidate_ blocks (potentially unfinalized) that are produced by the BFT consensus algorithm.
  Proto Blocks are full blocks _except_ that they don't contain any entropy (which is subsequently added by the Random Beacon)   
* **Random Beacon** - A _sub_-set of consensus nodes that generate entropy through byzantine-resilient protocol.
  The random beacon adds the entropy (byte-string) to the Proto Blocks.
* (Full) **Block** - a proto block which
  - has been finalized (committed) by the BFT consensus protocol
  - with added entropy by the Random Beacon       

## Details

### Collection Submission

Collection are submitted to (one or more) CN(s) via the `SubmitCollection` gRPC method.
  - During normal operations, Consensus nodes only receive the  _hashes_ 
    and (aggregated) signatures from the collectors that guarantee the collection.  
    The collection's content is _not_ resolved or inspected.  
  - CNs will only consider _guaranteed collection_ (see [Collection](../../../internal/roles/collect) for details)
Consensus nodes will gossip received collections to other consensus nodes.  

<!--
**Relevant packages:** [/internal/nodes/access/controllers](/internal/nodes/access/controllers)
-->

### Block Formation (and Mempool)

Consensus nodes store and track the status of all collection they receive. Each node maintains this sort of Mempool for itself.
The mempool only operates on the level of collections and provides the following functionality.
- Get list of all pending collections that are _not_ included in a finalized block (yet)
- Get the list of pending collections that are included in non-finalized blocks.
- Order collections by submission time. 
- For a given fork: get all pending collections that are not included in this specific fork 
- Collections that are included in a finalized block may be pruned from the mempool

When a CN generates a proto block, it includes all pending collections that are not in the current fork. 

When a CN generates sees a proto block from a different CN, it
* requests and verifies all collections it hoesn't have in its mempool (hashes and signatures only)
* updates the status of all collections in the block 
 
<!--
**Relevant packages:** [/internal/nodes/access/controllers](/internal/nodes/access/controllers)
-->
 

### BFT consensus

The BFT consensus algorithm is are abstracted behind a generic API. 
The only fixed API-level requirement for the consensus algorithm is deterministic finality.    

<!--
**Relevant packages:** [/internal/nodes/access/controllers](/internal/nodes/access/controllers)
-->

### Random Beacon

adds entropy for seeding pseudo-random-number generators to the block


### Block Publication

broadcasting the resulting finalized block to the entire network 

## Interaction Graph 
![interaction-flow](../../../docs/images/Consensus_interaction_flow.png?raw=true)
