package coldstuff

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
)

func NewFollower(log zerolog.Logger, state protocol.MutableState, me module.Local) *Follower {
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
	state protocol.MutableState
	me    module.Local
}

func (f *Follower) SubmitProposal(proposal *flow.Header, parentView uint64) {
	err := f.state.Finalize(proposal.ID())
	f.log.Err(err).Msg("cold stuff follower could not finalize proposal")
}

func (f *Follower) Ready() <-chan struct{} {
	return f.unit.Ready()
}

func (f *Follower) Done() <-chan struct{} {
	return f.unit.Done()
}
