# Observation API

## Entities

### Account

An account is a user's identity on the Flow blockchain. It contains a unique address, balance, a list of public keys and any code that has been deployed to the account. 

```
message Account {
  bytes address
  uint64 balance
  bytes code
  repeated AccountPublicKey keys
}
```

| Field        | Description    |
|--------------|----------------|
| address      | A unique account address |
| balance      | The account balance |
| code         | The code deployed to this account |
| keys         | A list of the public keys for this account |

### Account Public Key

An account public key is a reference to a public key associated with a Flow account. Accounts can be configured with zero or more public keys, each of which can be used for signature verification when authorizing a transaction.

```
message AccountPublicKey {
  bytes public_key
  uint32 sign_algo
  uint32 hash_algo
  uint32 weight
}
```

| Field         | Description    |
|---------------|----------------|
| public_key    | The public key encoded as bytes |
| sign_algo     | The signature scheme (currently ECDSA-P256 or ECDSA-SECp256k1) |
| hash_algo     | The hashing algorithm (SHA2-256 or SHA3-256) |
| weight        | The weight assigned to this key |

### Transaction

A transaction represents a unit of computation that is submitted to the Flow network. It is signed by one or more Flow accounts, and also specifies limits for the network and computation fees required for processing.

```
message Transaction {
  bytes script
  bytes reference_block_hash
  uint64 nonce
  uint64 compute_limit
  bytes payer_account
  repeated bytes script_accounts
  repeated AccountSignature signatures
  TransactionStatus status
}
```

| Field        | Description    |
| ------------- |---------------| 
| script               | Raw source code for a Cadence script, encoded as UTF-8 bytes |
| reference_block_hash | The reference block hash is the hash of an existing block used to specify the expiry window for this transaction. Transactions expire after `N` blocks are sealed on top of the specified reference hash |
| nonce                | An arbitrary nonce |
| compute_limit        | The gas limit for the transaction execution |
| payer_account        | The account that is paying for the gas and network fees of the transaction |
| script_accounts      | The accounts that have authorized the transaction to update their state. A transaction can have zero to many script accounts. |
| signatures           | Signature of either the payer or script accounts |
| status               | One of `unknown`, `pending`, `finalized`, or `sealed` |

Note: The total weight of the  keys of each of the signing account should be greater than or equal to 1000.

### Block

A block includes the transactions as well as the other inputs (incl. the random seed) required for execution, but not the resulting state after block execution.

A block may be sealed or unsealed depending on whether it has been computed and the resulting execution state has been verified.
 

```
message Block {
  string chain_id
  uint64 number
  bytes previous_block_hash
  google.protobuf.Timestamp timestamp
  repeated SignedCollectionHash signed_collection_hashes
  repeated BlockSeal block_seals
  repeated bytes signatures
}
```

| Field                    | Description    |
| -------------------------|----------------|
| chainID                  | Unique ID of the Flow blockchain |
| number                   | The number of the block in the chain |
| previous_block_hash      | Hash of the previous block in the chain |
| timestamp                | Timestamp when the block was proposed |
| signed_collection_hashes | A list of hashes of all the collections in the block |
| blockSeals               | List of block seal hashes |
| signatures               | Signatures of the consensus nodes |

### Block Header

A block header is a short description of a block and does not contain the full block contents. It contains the block hash and the hash of the previous block.

The latest sealed block header represents the last block that was added to the chain, while the latest unsealed header represents the last finalized block that has not yet been verified.

```
message BlockHeader {
  bytes hash
  bytes previous_block_hash
  uint64 number
}
```

| Field               | Description   |
|---------------------|---------------|
| hash                | The hash of the entire block payload, acts as the unique identifier for the block |
| previous_block_hash | Hash of the previous block in the chain |
| number              | The number of the block in the chain |

### Event

An event is emitted as the result of a transaction execution. Events can either be user-defined events originating from a Cadence smart contract, or Flow system events such as `AccountCreated`, `AccountUpdated`, etc.

More information on user-defined events can be found [here](https://github.com/dapperlabs/flow-go/tree/master/language/docs).

```
message Event {
  string type
  bytes transaction_hash
  uint32 index
  bytes payload
}
```
| Field            | Description    |
| -----------------|---------------| 
| type             | The fully-qualified unique type identifier of the event |
| transaction_hash | Hash of the transaction associated with this event |
| index            | index defines the ordering of events in a transaction. The first event emitted has index 0, the second has index 1, and so on |
| payload          |  Event fields encoded as XDR bytes.|

## RPC Service

### SendTransaction

`SendTransaction` submits a transaction to the network.

`rpc SendTransaction(SendTransactionRequest) returns (SendTransactionResponse)`

`SendTransaction` determines the correct cluster of collector nodes that is responsible for collecting the transaction based on the hash of the transaction and forwards the transaction to that cluster.

##### SendTransactionRequest

`SendTransactionRequest` message contains the transaction that is being request to be executed.

```
message SendTransactionRequest {
  Transaction transaction;
}
```

##### SendTransactionResponse

`SendTransactionResponse` message contains the hash of the submitted transaction.

```
message SendTransactionResponse {
  bytes hash = 1;
}
```

### GetTransaction

`GetTransaction` retrieves a transaction by hash. In addition to the transaction, it also returns all the events generated for that transaction.

```
rpc GetTransaction(GetTransactionRequest) returns (GetTransactionResponse);
```

`GetTransaction` queries an observation node for the specified transaction. If the transaction is not found in the observation node cache, the request is forwarded to a collection node.

_Currently, only transactions within the current epoch can be queried._

##### GetTransactionRequest

`GetTransactionRequest` contains the hash of the transaction that is being queried.

```
message GetTransactionRequest {
  bytes hash
}
```

##### GetTransactionResponse

`GetTransactionResponse` contains the transaction details and the events associated with the transaction.

```
message GetTransactionResponse {
  Transaction transaction
  Event events
}
```

### GetLatestBlock

`GetLatestBlock` returns the latest sealed or unsealed block as a block header.

```  
rpc GetLatestBlock(GetLatestBlockRequest) returns (GetLatestBlockResponse);
```

##### GetLatestBlockRequest

```
message GetLatestBlockRequest {
  bool is_sealed
}
``` 

##### GetLatestBlockResponse

```
message GetLatestBlockResponse {
  BlockHeader block
}
```

### GetAccount

`GetAccount` gets an account by address.

```
rpc GetAccount(GetAccountRequest) returns (GetAccountResponse)
```

`GetAccount` queries the execution node for the account details stored as part of the execution state.

##### GetAccountRequest

```
message GetAccountRequest {
  bytes address
}
```

#### GetAccountResponse

```
message GetAccountResponse {
  Account account
}
```

### ExecuteScript

`ExecuteScript` executes a read-only Cadance script against the latest sealed execution state.

```
rpc ExecuteScript(ExecuteScriptRequest) returns (ExecuteScriptResponse)
```

`ExecuteScript` can be used to read execution state from the Flow blockchain. The script is executed on an execution node and the return value is encoded as XDR bytes. 

##### ExecuteScriptRequest

```
message ExecuteScriptRequest {
  bytes script
}
```

##### ExecuteScriptResponse

```
message ExecuteScriptResponse {
  bytes value
}
```

### GetEvents

`GetEvents` retrieves events matching a given query.

```
rpc GetEvents(GetEventsRequest) returns (GetEventsResponse)
```

Events can be requested for a specific block range via the `start_block` and `end_block` (inclusive) fields and further filtered by the event type via the `type` field. Event types are namespaced with the address of the account and contract in which they are declared. Events are provided by execution nodes.

##### GetEventsRequest

```
message GetEventsRequest {
  string type
  uint64 start_block
  uint64 end_block
}
```

##### GetEventsResponse

```
message GetEventsResponse {
    repeated Event events
}
```

### Ping

`Ping` will return a successful response if the Observation API is ready and available.

```
rpc Ping(PingRequest) returns (PingResponse)
```

If a ping request returns an error or times out, it can be assumed that the Observation API is unavailable.

#### PingRequest

```
message PingRequest {
}
```

#### PingResponse

```
message PingResponse {
}
```
