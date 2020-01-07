# (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

using Go = import "/go.capnp";

$Go.package("captain");
$Go.import("github.com/dapperlabs/flow-go/schema/captain");

@0xcc8ede639915bf22;

using Trickle = import "trickle.capnp";
using Collection = import "collection.capnp";
using Coldstuff = import "coldstuff.capnp";

struct Message {
  union {
    auth @0 :Trickle.Auth;
    ping @1 :Trickle.Ping;
    pong @2 :Trickle.Pong;
    announce @3 :Trickle.Announce;
    request @4 :Trickle.Request;
    response @5 :Trickle.Response;

    collectionGuarantee @6 :Collection.CollectionGuarantee;

    blockProposal @7 :Coldstuff.BlockProposal;
    blockVote @8 :Coldstuff.BlockVote;
    blockCommit @9 :Coldstuff.BlockCommit;
  }
}
