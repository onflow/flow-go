# Observer Service

The observer service provides a read-only point of contact to interact with the Flow network. It implements parts of the [Access API](https://docs.onflow.org/access-api/). It only implements a subset. Users, who need to change the state of the network should opt for running an Access node.

It is a GRPC server which connects to a staked access node via libp2p, and forwards execute requests via GRPC. It is a scalable service.

At a high level it does the following:

1. Forwards all read-only Script related calls (`ExecuteScriptAtLatestBlock`, `ExecuteScriptAtBlockID` and `ExecuteScriptAtBlockHeight`) to one of the access nodes that forwards it to one of the execution services.
2. Follows updates to the blockchain and locally caches transactions, collections, and sealed blocks.
3. Replies to client API calls for information such as `GetBlockByID`, `GetCollectionByID`, `GetTransaction` etc.


***NOTE**: The Observer service does not participate in the Flow protocol*

