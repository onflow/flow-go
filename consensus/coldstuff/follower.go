package coldstuff

import (
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/state/protocol"
)

func NewFollower(log zerolog.Logger, state protocol.State, me module.Local) *Follower {
	return &Follower{
		unit:  engine.NewUnit(),
		log:   log,
		state: state,
		me:    me,
	}
}

type Follower struct {
	unit  *engine.Unit // used to manage concurrency & shutdown
	log   zerolog.Logger
	state protocol.State
	me    module.Local
}

func (f *Follower) SubmitProposal(proposal *flow.Header, parentView uint64) {
	f.state.Mutate().Finalize(proposal.ID())
}

func (f *Follower) Ready() <-chan struct{} {
	return f.unit.Ready()
}

func (f *Follower) Done() <-chan struct{} {
	return f.unit.Done()
}
