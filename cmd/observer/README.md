# Observer Node

The observer node provides a read-only point of contact to interact with the Flow network. It implements parts of the [Access API](https://docs.onflow.org/access-api/). It only implements a subset. Users, who need to change the state of the network should opt for running an Access Node.

It is a GRPC server which also connects to a staked access node or other observer nodes via GRPC. It is a scalable service.

At a high level it does the following:

1. Forwards all read-only Script related calls (`ExecuteScriptAtLatestBlock`, `ExecuteScriptAtBlockID` and `ExecuteScriptAtBlockHeight`) to one of the execution nodes.
2. Follows updates to the blockchain and locally caches transactions, collections, and sealed blocks.
3. Replies to client API calls for information such as `GetBlockByID`, `GetCollectionByID`, `GetTransaction` etc.


***NOTE**: The Observer node does not participate in the Flow protocol*
