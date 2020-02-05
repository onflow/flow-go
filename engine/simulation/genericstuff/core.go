package genericstuff

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine/simulation/coldstuff/round"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/protocol"
)

type ColdStuff interface {
	Start() (exit func(), done chan struct{})

	SubmitProposal(proposal *Proposal)
	SubmitVote(vote *Vote)
	SubmitCommit(commit *Commit)
}

// internal implementation of ColdStuff
type coldStuff struct {
	log     zerolog.Logger
	round   *round.Round
	builder module.Builder
	comms   Communicator

	interval time.Duration
	timeout  time.Duration

	proposals chan Proposal
	votes     chan Vote
	commits   chan Commit
}

func New(
	log zerolog.Logger,
	state protocol.State,
	me module.Local,
	builder module.Builder,
	comms Communicator,
	interval time.Duration,
	timeout time.Duration,
) (ColdStuff, error) {
	round, err := round.New(state, me)
	if err != nil {
		return nil, err
	}

	cold := coldStuff{
		log:       log,
		round:     round,
		builder:   builder,
		comms:     comms,
		interval:  interval,
		timeout:   timeout,
		proposals: make(chan Proposal, 1),
		votes:     make(chan Vote, 1),
		commits:   make(chan Commit, 1),
	}

	return &cold, nil
}

func (c *coldStuff) Start() (exit func(), done chan struct{}) {
	return nil, nil
}

func (c *coldStuff) SubmitProposal(proposal *Proposal) {

}

func (c *coldStuff) SubmitVote(vote *Vote) {

}

func (c *coldStuff) SubmitCommit(commit *Commit) {

}

func (c *coldStuff) loop() error {

}

func (c *coldStuff) sendProposal() error {
	return nil
}

func (c *coldStuff) waitForVotes() error {
	return nil
}

func (c *coldStuff) sendCommit() error {
	return nil
}

func (c *coldStuff) waitForPoposal() error {
	return nil
}

func (c *coldStuff) waitForCommit() error {
	return nil
}

func (c *coldStuff) commitCandidate() error {
	return nil
}
