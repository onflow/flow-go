package voter

import (
	"github.com/dapperlabs/flow-go/engine/consensus/eventdriven/modules/def"
	"go.uber.org/atomic"
)

// Voter contains the logic for deciding whether to vote or not on a safe block and emitting the vote.
// Specifically, Voter processes on OnSafeBlock events from reactor/core/eventprocessor
type Voter struct {
	// enable or disable Voter, e.g. for when catching up blocks
	enabled atomic.Bool
}

func NewVoter(enabled bool) *Voter {
	return &Voter{enabled: *atomic.NewBool(enabled)}
}

func (v *Voter) Enable()  { v.enabled.Store(true) }
func (v *Voter) Disable() { v.enabled.Store(false) }

// OnSafeBlock votes once Reactor has deemed that a block should be voted on
func (v *Voter) OnSafeBlock(block *def.Block) {
	go v.vote(block)
}

// vote sends a vote to the next primary if Voter is enabled
func (v *Voter) vote(block *def.Block) {
	if !v.enabled.Load() {
		return
	}
	panic("Implement rest")
}

func (v *Voter) OnEnteringView(view uint64) {
	panic("Implement me")
}
