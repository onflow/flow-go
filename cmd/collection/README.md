# Collection 

The collection node is responsible for accepting transactions from users, packaging 
them into collections to ease the load on consensus nodes, and storing transaction
texts for the duration they are needed by the network.

This document provides a high-level overview of the collection node. Each section
includes links to the appropriate package, which may contain more detailed documentation.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
## Table of Contents

- [Terminology](#terminology)
- [Engines](#engines)
  - [Ingest](#ingest)
  - [Proposal](#proposal)
  - [Synchronization](#synchronization)
  - [Provider](#provider)
- [Storage](#storage)
  - [Cluster State](#cluster-state)
    - [Collection Builder](#collection-builder)
    - [Collection Finalizer](#collection-finalizer)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Terminology

* **Collection** - a set of transactions proposed by a cluster of collection nodes.
* **Guaranteed Collection** - a collection that a quorum of nodes in the cluster has
  committed to storing. 
* **Collection Guarantee** - the attestation to a collection that has been guaranteed.
  Concretely, this is a hash over the collection and signatures from a quorum of 
  cluster members. (Sometimes simply referred to as `guarantee`.)
* **Cluster** - a group of collection nodes that work together to create collections.
  Each cluster is responsible for a different subset of transactions.

## Engines

### [Ingest](../../engine/collection/ingest)

The `ingest` engine is responsible for accepting, validating, and storing new transactions. 
Once a transaction has been ingested, it can be included in a new collection via the `proposal` engine.

In general, collection nodes _cannot_ fully ensure that a transaction is valid. 
Consequently, the validation performed at this stage is best-effort.

### [Proposal](../../engine/collection/proposal)

The `proposal` engine is responsible for handling the consensus process for the cluster. 
It runs an instance of [HotStuff](../../consensus/hotstuff) within the cluster. 
This results in each cluster building a secondary blockchain, where each block 
represents a proposed collection.

### [Synchronization](../../engine/collection/synchronization)

The `synchronization` engine is responsible for keeping the node in sync with its cluster.
It periodically polls cluster members for their latest finalized collection, and handles
requesting ranges of finalized collections when the node is behind. It also handles 
requesting specific collections when the `proposal` engine receives a new proposal for
which the parent is not known.

### [Provider](../../engine/collection/provider)

The `provider` engine is responsible for responding to requests for resources stored
by the collection node, typically full collections or individual transactions. In
particular, execution nodes and verification nodes request collections in order to
execute the constituent transactions.

## Storage

### [Cluster State](../../state/cluster)

The cluster state implements a storage layer for the blockchain produced by the 
`proposal` engine. The design mirrors the [Protocol State](../../state/protocol),
which is the storage layer for the main Flow blockchain produced by consensus nodes.

Since the core HotStuff logic is only aware of blocks, we sometimes use the term 
"cluster block" to refer to a block within a cluster's auxiliary blockchain. Each
cluster block contains a single proposed collection.

#### [Collection Builder](../../module/builder/collection)

The collection builder implements the logic for building new, valid collections
to propose to the cluster.

Valid collections contain transactions that have passed basic validation, are not
expired, and do not contain any transactions that already exist in a parent collection.

#### [Collection Finalizer](../../module/finalizer/collection)

The collection finalizer marks a collection as finalized (aka guaranteed). This is
invoked by the core [HotStuff](../../consensus/hotstuff) logic when it determines
that a collection has been finalized by the cluster.

When a collection is finalized, the corresponding collection guarantee is sent to
consensus nodes for inclusion in a block.
