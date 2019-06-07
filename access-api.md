# Bamboo Access API

The Bamboo Access API is the user-facing RPC API provided by Bamboo access nodes.

The spec for this API is defined using [Protocol Buffers](https://developers.google.com/protocol-buffers), and aims to be language agnostic 
in order to support a variety of server and client implementations.

## SendTransaction

Submit a transaction to the network.

```protobuf
rpc SendTransaction(SendTransactionRequest) returns (SendTransactionResponse);

message SendTransactionRequest {
  message Transaction {
    bytes to = 1;
    bytes data = 2;
    uint64 nonce = 3;
    uint64 compute = 4;
    bytes payerSignature = 5;
  }
  Transaction transaction = 1;
}

message SendTransactionResponse {
  bytes hash = 1;
}
```

## GetBlockByHash

Get a block by its hash.

```protobuf
rpc GetBlockByHash(GetBlockByHashRequest) returns (GetBlockByHashResponse);

message Block {
  enum Status {
    PENDING = 0;
    SEALED = 1;
  }
  bytes hash = 1;
  uint64 number = 2;
  repeated bytes transactionHashes = 3;
  Status status = 4;
}

message GetBlockByHashRequest {
  bytes hash = 1;
}

message GetBlockByHashResponse {
  Block block = 1;
}
```

## GetBlockByNumber

Get a block by its number (height).

```protobuf
rpc GetBlockByNumber(GetBlockByNumberRequest) returns (GetBlockByNumberResponse);

message Block {
  enum Status {
    PENDING = 0;
    SEALED = 1;
  }
  bytes hash = 1;
  uint64 number = 2;
  repeated bytes transactionHashes = 3;
  Status status = 4;
}

message GetBlockByNumberRequest {
  bytes number = 1;
}

message GetBlockByNumberResponse {
  Block block = 1;
}
```

## GetLatestBlock

Get the latest (sealed or unsealed) block produced by the network.

```protobuf
rpc GetLatestBlock(GetLatestBlockRequest) returns (GetLatestBlockResponse);

message Block {
  enum Status {
    PENDING = 0;
    SEALED = 1;
  }
  bytes hash = 1;
  uint64 number = 2;
  repeated bytes transactionHashes = 3;
  Status status = 4;
}

message GetLatestBlockRequest {
    bool isSealed = 1;
}

message GetLatestBlockResponse {
  Block block = 1;
}
```

## GetTransaction

Get a transaction by hash.

```protobuf
rpc GetTransaction(GetTransactionRequest) returns (GetTransactionResponse);

message GetTransactionRequest {
  bytes hash = 1;
}

message GetTransactionResponse {
  message Transaction {
    enum Status {
      PENDING = 0;
      FINALIZED = 1;
      REVERTED = 2;
      SEALED = 3;
    }
    bytes to = 1;
    bytes data = 2;
    uint64 nonce = 3;
    uint64 compute = 4;
    bytes payerSignature = 5;
    Status status = 6;
  }
  Transaction transaction = 1;
}
```

## GetBalance

Get the balance of an address.

```protobuf
rpc GetBalance(GetBalanceRequest) returns (GetBalanceResponse);

message GetBalanceRequest {
  bytes address = 1;
}

message GetBalanceResponse {
  bytes value = 1;
}
```

## CallContract

Perform a contract call.

```protobuf
rpc CallContract(CallContractRequest) returns (CallContractResponse);

message CallContractRequest {
  bytes data = 1;
}

message CallContractResponse {
  bytes data = 1;
}
```
