# (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

using Go = import "/go.capnp";

$Go.package("captain");
$Go.import("github.com/dapperlabs/flow-go/schema/captain");

@0xca5023959d73b9b5;

struct CollectionGuarantee {
  hash @0 :Data;
  signature @1 :Data;
}
