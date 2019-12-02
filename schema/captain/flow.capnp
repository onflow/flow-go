# (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

using Go = import "/go.capnp";

$Go.package("captain");
$Go.import("github.com/dapperlabs/flow-go/schema/captain");

@0xcf3ad8085685b22d;

struct BlockHeader {
  height @0 :UInt64;
  nonce @1 :UInt64;
  timestamp @2 :UInt64;
  parent @3 :Data;
  payload @4 :Data;
}
