package voterEvents

import "github.com/dapperlabs/flow-go/engine/consensus/HotStuff/modules/defConAct"

type Processor interface {
	OnSentVote(*defConAct.Vote)
}

type SentVoteConsumer interface {
	OnSentVote(*defConAct.Vote)
}
