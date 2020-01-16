# (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

using Go = import "/go.capnp";

$Go.package("captain");
$Go.import("github.com/dapperlabs/flow-go/schema/captain");

using Collection = import "collection.capnp";

@0xcf3ad8085685b22d;

struct Identity {
  nodeId @0 :Data;
  address @1 :Text;
  role @2 :UInt8;
  stake @3 :UInt64;
}

struct Header {
  number @0 :UInt64;
  timestamp @1 :UInt64;
  parent @2 :Data;
  payload @3 :Data;
  signatures @4 :List(Data); 
}

struct Block {
  header @0 :Header;
  newIdentities @1 :List(Identity);
  collectionGuarantees @2 :List(Collection.CollectionGuarantee);
}
