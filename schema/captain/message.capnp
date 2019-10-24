# (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

using Go = import "/go.capnp";

$Go.package("captain");
$Go.import("github.com/dapperlabs/flow-go/schema/captain");

@0xcc8ede639915bf22;

using Trickle = import "trickle.capnp";
using Collection = import "collection.capnp";
using Consensus = import "consensus.capnp";

struct Message {
  union {
    auth @0 :Trickle.Auth;
    ping @1 :Trickle.Ping;
    pong @2 :Trickle.Pong;
    announce @3 :Trickle.Announce;
    request @4 :Trickle.Request;
    response @5 :Trickle.Response;

    guaranteedCollection @6: Collection.GuaranteedCollection;

    snapshotRequest @7: Consensus.SnapshotRequest;
    snapshotResponse @8: Consensus.SnapshotResponse;
    mempoolRequest @9: Consensus.MempoolRequest;
    mempoolResponse @10: Consensus.MempoolResponse;
  }
}
