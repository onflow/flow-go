# Bamboo Access API

## SendTransaction

Submit a transaction to the network.

```protobuf
rpc SendTransaction(SendTransactionRequest) returns (SendTransactionResponse);

message SendTransactionRequest {
  message Transaction {
    bytes data = 1;
    uint64 nonce = 2;
    bytes payerSignature = 3;
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

message GetBlockByHashRequest {
  bytes hash = 1;
}

message GetBlockByHashResponse {
  message Block {
    bytes hash = 1;
    uint64 number = 2;
    repeated bytes transactionHashes = 3;
  }
  Block block = 1;
}
```

## GetBlockByNumber

Get a block by its number (height).

```protobuf
rpc GetBlockByNumber(GetBlockByNumberRequest) returns (GetBlockByNumberResponse);

message GetBlockByNumberRequest {
  bytes hash = 1;
}

message GetBlockByNumberResponse {
  message Block {
    bytes hash = 1;
    uint64 number = 2;
    repeated bytes transactionHashes = 3;
  }
  Block block = 1;
}
```

## GetLatestBlock

Get the latest (sealed or unsealed) block produced by the network.

```protobuf
rpc GetLatestBlock(GetLatestBlockRequest) returns (GetLatestBlockResponse);

message GetLatestBlockRequest {
    bool isSealed = 1;
}

message GetLatestBlockResponse {
  message Block {
    bytes hash = 1;
    uint64 number = 2;
    repeated bytes transactionHashes = 3;
  }
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
  enum Status {
    PENDING = 0;
    IN_BLOCK = 1;
    SEALED = 2;
  }
  message Transaction {
    bytes data = 1;
    uint64 nonce = 2;
    bytes payerSignature = 3;
    Status status = 4;
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
  bytes data = 2;
}
```