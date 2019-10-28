# (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

using Go = import "/go.capnp";

$Go.package("captain");
$Go.import("github.com/dapperlabs/flow-go/schema/captain");

@0x9691a973e8a27b6b;

using Collection = import "collection.capnp";

struct SnapshotRequest {
  nonce @0 :UInt64;
  mempoolHash @1 :Data;
}

struct SnapshotResponse {
  nonce @0 :UInt64;
  mempoolHash @1 :Data;
}

struct MempoolRequest {
  nonce @0 :UInt64;
}

struct MempoolResponse {
  nonce @0 :UInt64;
  collections @1 :List(Collection.GuaranteedCollection);
}
