package inmem

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// DynamicProtocolStateAdapter implements protocol.DynamicProtocolState by wrapping an InitialProtocolStateAdapter.
type DynamicProtocolStateAdapter struct {
	InitialProtocolStateAdapter
	params protocol.GlobalParams
}

var _ protocol.DynamicProtocolState = (*DynamicProtocolStateAdapter)(nil)

func NewDynamicProtocolStateAdapter(entry *flow.RichProtocolStateEntry, params protocol.GlobalParams) *DynamicProtocolStateAdapter {
	return &DynamicProtocolStateAdapter{
		InitialProtocolStateAdapter: InitialProtocolStateAdapter{
			RichProtocolStateEntry: entry,
		},
		params: params,
	}
}

func (s *DynamicProtocolStateAdapter) Identities() flow.IdentityList {
	return s.RichProtocolStateEntry.CurrentEpochIdentityTable
}

func (s *DynamicProtocolStateAdapter) GlobalParams() protocol.GlobalParams {
	return s.params
}

// InvalidEpochTransitionAttempted denotes whether an invalid epoch state transition was attempted
// on the fork ending this block. Once the first block where this flag is true is finalized, epoch
// fallback mode is triggered.
// TODO for 'leaving Epoch Fallback via special service event': at the moment, this is a one-way transition and requires a spork to recover - need to revisit for sporkless EFM recovery
func (s *DynamicProtocolStateAdapter) InvalidEpochTransitionAttempted() bool {
	return s.ProtocolStateEntry.InvalidEpochTransitionAttempted
}

// PreviousEpochExists returns true if a previous epoch exists. This is true for all epoch
// except those immediately following a spork.
func (s *DynamicProtocolStateAdapter) PreviousEpochExists() bool {
	return s.PreviousEpoch != nil
}

// EpochPhase returns the epoch phase for the current epoch.
func (s *DynamicProtocolStateAdapter) EpochPhase() flow.EpochPhase {
	return s.Entry().EpochPhase()
}
