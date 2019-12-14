package reactor

import (
	"github.com/dapperlabs/flow-go/engine/consensus/componentsreactor/core"
	coreevents "github.com/dapperlabs/flow-go/engine/consensus/componentsreactor/core/events"
	"github.com/dapperlabs/flow-go/engine/consensus/componentsreactor/forkchoice"
	forkchoiceevents "github.com/dapperlabs/flow-go/engine/consensus/componentsreactor/forkchoice/events"
	"github.com/dapperlabs/flow-go/engine/consensus/componentsreactor/pacemaker"
	pacemakerevents "github.com/dapperlabs/flow-go/engine/consensus/componentsreactor/pacemaker/events"
	"github.com/dapperlabs/flow-go/engine/consensus/componentsreactor/voteAggregator"
	voteAggregatorEvents "github.com/dapperlabs/flow-go/engine/consensus/componentsreactor/voteAggregator/events"
)

func main() {
	reactorEventProc := coreevents.NewPubSubEventProcessor()
	finalizer := core.New(nil, nil, reactorEventProc)

	fcEventProc := forkchoiceevents.NewPubSubEventProcessor()
	forkchoice := forkchoice.NewNewestForkChoice(finalizer, fcEventProc)

	reactor := NewReactor(finalizer, forkchoice)

	var pmEventProc pacemakerevents.Processor
	pmEventProc = nil // pacemakerevents.NewPubSubEventProcessor()

	var pm pacemaker.Pacemaker
	pm = nil

	reactorEventProc.AddIncorporatedBlockProcessor(pm)
	pmEventProc.AddBlockProposalTriggerProcessor(reactor)

	pm.Run()
	reactor.Run()

	vaPubSub := voteAggregatorEvents.New()
	va := voteAggregator.VoteAggregator(vaPubSub)

	vaPubSub.AddQcFromVotesConsumer(reactor)

}
