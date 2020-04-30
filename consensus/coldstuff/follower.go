package coldstuff

import (
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
)

func NewFollower(log zerolog.Logger, final module.Finalizer, me module.Local) *Follower {
	return &Follower{
		unit:  engine.NewUnit(),
		log:   log,
		final: final,
		me:    me,
	}
}

type Follower struct {
	unit  *engine.Unit // used to manage concurrency & shutdown
	log   zerolog.Logger
	final module.Finalizer
	me    module.Local
}

func (f *Follower) SubmitProposal(proposal *flow.Header, parentView uint64) {
	err := f.final.MakeFinal(proposal.ID())
	f.log.Err(err).Msg("cold stuff follower could not finalize proposal")
}

func (f *Follower) Ready() <-chan struct{} {
	return f.unit.Ready()
}

func (f *Follower) Done() <-chan struct{} {
	return f.unit.Done()
}
