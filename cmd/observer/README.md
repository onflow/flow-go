# Observer Node

The observer node provides a read-only point of contact to interact with the Flow network. It implements parts of the [Access API](https://docs.onflow.org/access-api/). It only implements a subset. Users, who need to change the state of the network should opt for running an Access Node.

It is a GRPC server which also connects to a staked access node or other observer nodes via GRPC. It is a scalable service.

At a high level it does the following:

1. Forwards all read-only Script related calls (`ExecuteScriptAtLatestBlock`, `ExecuteScriptAtBlockID` and `ExecuteScriptAtBlockHeight`) to one of the execution nodes.
2. Follows updates to the blockchain and locally caches transactions, collections, and sealed blocks.
3. Replies to client API calls for information such as `GetBlockByID`, `GetCollectionByID`, `GetTransaction` etc.


***NOTE**: The Observer node does not participate in the Flow protocol*

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Terminology](#terminology)
- [Processes](#processes)
  - [Transaction Lifecycle](#transaction-lifecycle)
- [Engines](#engines)
  - [Follower Engine](#follower-engine)
  - [Ingestion](#ingestion)
  - [Requester](#requester)
  - [RPC](#rpc)
  - [Ping](#ping)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Terminology

- **Transaction** - a transaction represents a unit of computation that is submitted to the Flow network.
- **Collection** - a set of transactions proposed by a cluster of collection nodes.
- **Header**, also _Block Header_ - a data structure containing the meta-data for a block, including the Merkle root hash for the payload as well as the relevant consensus node signatures.
- **Block** - the combination of a block header with block contents, representing all the data necessary to construct and validate the entirety of the block.

## Processes

### Transaction Lifecycle
1. Transactions are received by the access node via the [SendTransaction API call](https://docs.onflow.org/access-api/#sendtransaction).
2. The access node forwards the transaction to one of the Collection node in the Collection node cluster to which this transaction belongs to and stores it locally as well.
3. If a [GetTransaction](https://docs.onflow.org/access-api/#gettransaction) request is received, it is forwarded to the closest Access Node. The transaction is read from local storage and returned if found.
4. If a [GetTransactionResult](https://docs.onflow.org/access-api/#gettransactionresult) request is received, it is forwarded to the closest Access Node.
An execution node is requested for events for the transaction and the transaction status is derived as follows:
    1. If the collection containing the transaction and the block containing that collection is found locally, but the transaction has expired then its status is returned as `expired`.
    2. If either the collection or the block is not found locally, but the transaction has not expired, then its status is returned as `pending`
    3. If the transaction has neither expired nor is it pending, but the execution node has not yet executed the transaction,
       then the status of the transaction is returned as `finalized`.
    4. If the execution node has executed the transaction, then if the height of the block containing the transaction is greater than the highest sealed block,
    then the status of the transaction is returned as `executed` else it is returned as `sealed`.
    5. If the collection, block, or chain state lookup failed then the status is returned as `unknown`.


## Engines

Engines are units of application logic that are generally responsible for a well-isolated process that is part of the bigger system. They receive messages from the network on selected channels and submit messages to the network on the same channels.

### [Follower Engine](../../engine/common/follower)

The Follower engine follows the consensus progress and notifies the `ingestion` engine of any new finalized block.

### [Ingestion](../../engine/access/ingestion)

The `ingestion` engine receives finalized blocks from the `follower` engine and request all the collections for the block via the `requester` engine.
As the collections arrive, it persists the collections and the transactions within the collection in the local storage.

### [Requester](../../engine/common/requester)

The `requester` engine requests collections from the collection nodes on behalf of the `ingestion` engine.

### [RPC](../../engine/access/rpc)

The `rpc` engine is the GRPC server which responds to the [Access API](https://docs.onflow.org/access-api/) requests from clients.
It also supports GRPCWebproxy requests.

### [Ping](../../engine/access/ping)

The `ping` engine pings all the other nodes specified in the identity list via a [libp2p](https://github.com/libp2p/go-libp2p) ping and reports via metrics if the node is reachable or not.
This helps identify nodes in the system which are unreachable.


### Observer node sequence diagram

TODO

![Access node sequence diagram](/docs/AccessNodeSequenceDiagram.png)
