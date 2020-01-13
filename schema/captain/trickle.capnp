# (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

using Go = import "/go.capnp";

$Go.package("captain");
$Go.import("github.com/dapperlabs/flow-go/schema/captain");

@0xf71cc0af2f870b3c;

struct Auth {
  nodeId @0 :Data;
}

struct Ping {
  nonce @0 :UInt32;
}

struct Pong {
  nonce @0 :UInt32;
}

struct Announce {
  channelId @0 :UInt8;
  eventId @1 :Data;
}

struct Request {
  channelId @0 :UInt8;
  eventId @1 :Data;
}

struct Response {
  channelId @0 :UInt8;
  eventId @1 :Data;
  originId @2 :Data;
  targetIds @3 :List(Data);
  payload @4 :Data;
}
