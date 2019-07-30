# Collection

The collection stream concerns all aspects of the transaction flow from the moment a transaction is submitted to the network until it enters the consensus flow. 

More specifically, collection covers the following:

- Transaction submissions from user agents
- Transaction routing
- Cluster formation
- Collection building, consensus and signing
- Publishing collections to security nodes
- Transaction and collection storage

All collection duties are performed by access nodes.

## Terminology

* **Access Node (AN)** - The user-facing node archetype
* **Transaction** - A user-submitted request to execute code on the blockchain
* **Node Stake** - The amount a node has staked on its reputation within the network
* **Epoch** - A fixed period of time (measured in blocks) during which node stakes are fixed
* **AN Cluster** - A grouping of ANs that work together to form collections
* **Collection** - A set of transactions bundled together for execution
* **Collection Guarantor**: An AN that has signed a collection, thereby guaranteeing to store the collection and all contained transactions until execution (or until all transactions in the collection have expired).
* **Guaranteed Collection**: a collection where the accumulated stake of all guarantors is _more_ than 2/3 of the clusters total stake

## Cluster Formation

AN clusters are formed at the beginning of each epoch using a deterministic clustering algorithm. 

The algorithm returns a clustering arrangement that places roughly equal stake in each cluster. Higher-staked ANs may belong to multiple clusters.

The algorithm uses the block hash of the last block in the previous epoch as a source of entropy.

Details about the algorithm can be found in the package below:

**Relevant packages:** [/internal/protocol/collect/clusters](/internal/protocol/collect/clusters)


## Transaction Submission

Transactions are submitted to an access node via the `SendTransaction` gRPC method.

The access node validates the transaction and will return an error to the user in the following cases:

- Transaction is malformed
- Transaction is a duplicate
- Transaction has a missing/invalid signature
- Transaction is signed by an account that does not exist

**Relevant packages:** [/internal/nodes/access/controllers](/internal/nodes/access/controllers)



## Transaction Routing

Each transaction is routed to a cluster using a deterministic routing algorithm. This means that routing is verifiable; in other words, the same transaction will always be routed to the same cluster.

After a transaction is validated, it is forwarded to a node in the correct cluster. If the access node that received the transaction is already in the correct cluster, no forwarding is needed.

Once a transaction reaches a node in the correct cluster, that node will store the transaction and begin sharing it with other cluster peers.

**Relevant packages:** [/internal/roles/collect/routing](../../../internal/roles/collect/routing)

## Collection Building

The primary function of a cluster is to produce collections. Collections are formed through an established BFT consensus protocol (probably HotStuff). 

Each AN shares transactions within their cluster to create a shared pool of pending transactions. Eventually this pool is used to create a collection, which is organized by a collection owner.

The collection building process requires a sufficient number of ANs to sign each collection. We refer to the ANs that sign the collection as the collection guarantors. Only ANs that are part of the cluster that generates the collection can guarantee the collection.  
Specifically, we require that the accumulated stake of all guarantors is _more_ than 2/3 of the cluster's total stake. A collection with this property is a _guaranteed collection_.

Details about the collection building algorithm can be found in the package below:

**Relevant packages:** [/internal/roles/collect/collections](../../../internal/roles/collect/collections)

## Collection Publishing

After a collection is formed, the collection owner will send the collection to one or more security nodes to be included in a block.
Consensus nodes will only accept guaranteed collections.

**Dependency: this part of the flow interacts with the [Consensus](../../../internal/roles/consensus/) stream.**


**Relevant packages:** [/internal/roles/collect/collections](../../../internal/roles/collect/collections), [/internal/nodes/security/controllers](/internal/nodes/security/controllers)

## Transaction and Collection Storage

ANs are responsible for saving all transactions and collections that they commit to storing.

**Relevant packages:** [/internal/nodes/access/data](/internal/nodes/access/data)
