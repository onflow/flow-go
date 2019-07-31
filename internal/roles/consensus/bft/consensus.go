package bft

import "github.com/golang/protobuf/proto"

type Network interface {
	NodeCount() int
	NodeId(address string) int
	Broadcast(message proto.Message) // HELP: how do we specify set of receivers with default all
}

type ConsensusNetwork interface {
	Network
	RequestBlock(blockhash Protoblock, nodeId int, callback) // HELP: how do we do callback?
}

type Stake interface {
	Stake(address Address, block Protoblock) int // HELP: AttoBam = 10^-18
	TotalStake(block Protoblock) int
}

type Mempool interface {

}


type BftConsensus interface {
	process(message Messageable)
}

/* QUESTIONS:
 * How do abstract network layer?
 * how do you do a callback? (Interface?)
 * can we abstract the different messages that the consensus nodes are sending _only amongst consensus nodes_ ?



 * how do I implement abstract base class?
   -> when block finalized, feed it in callback
      do I need to reimplement this in for ecvery

*/

