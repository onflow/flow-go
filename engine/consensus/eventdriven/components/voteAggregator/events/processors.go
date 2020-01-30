package events

import (
	"github.com/dapperlabs/flow-go/engine/consensus/eventdriven/modules/def"
	"github.com/dapperlabs/flow-go/engine/consensus/eventdriven/modules/defConAct"
)

// Processor consumes events produced by voteAggregator
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type Processor interface {
	OnDoubleVote(*defConAct.Vote, *defConAct.Vote)
	OnInvalidVote(*defConAct.Vote)
	OnQcFromVotes(*def.QuorumCertificate)
}

type DoubleVoteConsumer interface {
	OnDoubleVote(*defConAct.Vote, *defConAct.Vote)
}

type InvalidVoteConsumer interface {
	OnInvalidVote(*defConAct.Vote)
}

type QcFromVotesConsumer interface {
	OnQcFromVotes(*def.QuorumCertificate)
}
