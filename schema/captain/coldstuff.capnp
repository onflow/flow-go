# (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

using Go = import "/go.capnp";

$Go.package("captain");
$Go.import("github.com/dapperlabs/flow-go/schema/captain");

using Flow = import "flow.capnp";

@0xca03600dec38188d;

struct BlockProposal {
  block @0 :Flow.Block;
}

struct BlockVote {
  hash @0 :Data;
}

struct BlockCommit {
  hash @0 :Data;
}
