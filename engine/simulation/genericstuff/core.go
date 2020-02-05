package genericstuff

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine/simulation/coldstuff/round"
	"github.com/dapperlabs/flow-go/module"
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
	round   round.Round
	builder module.Builder
	comms   Communicator

	interval time.Duration
	timeout  time.Duration

	proposals chan Proposal
	votes     chan Vote
	commits   chan Commit
}

func (c *coldStuff) Start() (exit func(), done chan struct{}) {
	return nil, nil
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
