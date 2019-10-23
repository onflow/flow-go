# (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

using Go = import "/go.capnp";

$Go.package("captain");
$Go.import("github.com/dapperlabs/flow-go/schema/captain");

@0xf71cc0af2f870b3c;

struct Auth {
  nodeId @0 :Text;
}

struct Ping {
  nonce @0 :UInt32;
}

struct Pong {
  nonce @0 :UInt32;
}

struct Announce {
  engineId @0: UInt8;
  eventId @1 :Data;
}

struct Request {
  engineId @0: UInt8;
  eventId @1 :Data;
}

struct Response {
  engineId @0 :UInt8;
  eventId @1 :Data;
  originId @2 :Text;
  targetIds @3 :List(Text);
  payload @4 :Data;
}
