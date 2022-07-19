# Observer Service

The observer service provides read-only access to the Flow network. It implements a subset of the [Access API](https://docs.onflow.org/access-api/) which is served from a local copy of the protocol data. It can be configured to proxy the remaining endpoints to an upstream staked Access Node.

At a high level it does the following:

1. Follows updates to the Flow network's protocol state, and maintains a local copy.

2. Runs a gRPC server which implements a subset of the Access API

    1. Responds to API calls for information such as `GetBlockByID`, `GetCollectionByID`, `GetTransaction` etc using its local data

    2. Forwards requests for execution state (`ExecuteScriptAt*`, `GetEvents*`, `TransactionResult`, etc) to a configured upstream staked Access Node.

***NOTE**: The Observer service does not participate in the Flow protocol*


