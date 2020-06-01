# Collection 

The collection node is responsible for accepting transactions from users, packaging 
them into collections to ease the load on consensus nodes, and storing transaction
texts for the duration they are needed by the network.

This document provides a high-level overview of the collection node. Each section
includes links to the appropriate package, which will contain more detailed
documentation if applicable.

## Terminology

* **Collection** - a set of transactions proposed by a cluster of collection nodes.
* **Guaranteed Collection** - a collection which a quorum of nodes in the cluster has
  committed to storing. When we are referring only to the commitment, not the full 
  collection, this is referred to as a **collection guarantee** or simply **guarantee**.
* **Cluster** - a group of collection nodes that work together to create collections.
  Each cluster is responsible for a different subset of transactions

## Engines

### [Ingest](../../engine/collection/ingest)

The `ingest` engine is responsible for accepting, validating, and storing new transactions. 
Once a transaction has been ingested, it can be included in a new collection via the `proposal` engine.

### [Proposal](../../engine/collection/proposal)

The `proposal` engine is responsible for handling the consensus process for the cluster. 
It runs an instance of [HotStuff](../../consensus/hotstuff) within the cluster. 
This results in each cluster building a tertiary blockchain, where each block 
represents a proposed collection.

### [Provider](../../engine/collection/provider)

The `provider` engine is responsible for responding
to requests for resources stored by the collection node, typically full collections or
individual transactions.

## Storage

### [Cluster State](../../state/cluster)

The cluster state implements a storage layer for the blockchain produced by the 
`proposal` engine. The design mirrors the [Protocol State](../../state/protocol),
which is the storage layer for the main Flow blockchain produced by consensus nodes.

#### [Collection Builder](../../module/builder/collection)

The collection builder implements the logic for building new, valid collections
to propose to the cluster.

#### [Collection Finalizer](../../module/finalizer/collection)

The collection finalizer marks a collection as finalized (aka guaranteed). This is
invoked by the core [HotStuff](../../consensus/hotstuff) logic when it determines
that a collection has been finalized by the cluster.

## Misc

* Transaction/Collection expiry
* Transaction Flow
