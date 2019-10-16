# (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

using Go = import "/go.capnp";

@0xd168b59a52c33ef7;

$Go.package("capnp");
$Go.import("github.com/dapperlabs/flow-go/pkg/module/codec/capnp");

struct Z {
  union {
    ping @0 :Ping;
    pong @1 :Pong;
    auth @2: Auth;
    announce @3: Announce;
    request @4: Request;
    event @5: Event;

    collection @6: Collection;
    receipt @7: Receipt;
    approval @8: Approval;
    seal @9: Seal;

    block @10: Block;
    vote @11: Vote;
    timeout @12: Timeout;
  }
}

struct Ping {
  nonce @0 :UInt32;
}

struct Pong {
  nonce @0 :UInt32;
}

struct Auth {
  node @0 :Text;
}

struct Announce {
  engine @0: UInt8;
  id @1 :Data;
}

struct Request {
  engine@0: UInt8;
  id @1 :Data;
}

struct Event {
  engine @0 :UInt8;
  payload @1 :Data;
}



struct Collection {
}

struct Receipt {
}

struct Approval {
}

struct Seal {
}


struct Block {
}

struct Vote {
}

struct Timeout {
}
