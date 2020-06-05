# Access Node

The access node provides a single point of contact to interact with the Flow network. It implements the [Access API](https://github.com/onflow/flow/blob/master/docs/access-api-spec.md)

It is a GRPC server which also connects to a collection node and an execution node via GRPC.

At a high level it does the following:

1. Forwards transaction received from the client via the `SendTransaction` call to the collection node.
2. Forwards all Script related calls (`ExecuteScriptAtLatestBlock`, `ExecuteScriptAtBlockID` and `ExecuteScriptAtBlockHeight`) to one of the execution node
3. Follows updates to the blockchain and locally caches transactions, collections, and sealed blocks.
4. Reply to client API calls for information such as `GetBlockByID`, `GetCollectionByID`, `GetTransaction` etc.


***NOTE**: The Access node does not participate in the Flow protocol*
