package inmem

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// EpochProtocolStateAdapter implements protocol.EpochProtocolState by wrapping an InitialProtocolStateAdapter.
type EpochProtocolStateAdapter struct {
	*flow.RichProtocolStateEntry
	params protocol.GlobalParams
}

var _ protocol.EpochProtocolState = (*EpochProtocolStateAdapter)(nil)

func NewDynamicProtocolStateAdapter(entry *flow.RichProtocolStateEntry, params protocol.GlobalParams) *EpochProtocolStateAdapter {
	return &EpochProtocolStateAdapter{
		RichProtocolStateEntry: entry,
		params:                 params,
	}
}

func (s *EpochProtocolStateAdapter) Epoch() uint64 {
	return s.CurrentEpochSetup.Counter
}

func (s *EpochProtocolStateAdapter) Clustering() (flow.ClusterList, error) {
	clustering, err := ClusteringFromSetupEvent(s.CurrentEpochSetup)
	if err != nil {
		return nil, fmt.Errorf("could not extract cluster list from setup event: %w", err)
	}
	return clustering, nil
}

func (s *EpochProtocolStateAdapter) EpochSetup() *flow.EpochSetup {
	return s.CurrentEpochSetup
}

func (s *EpochProtocolStateAdapter) EpochCommit() *flow.EpochCommit {
	return s.CurrentEpochCommit
}

func (s *EpochProtocolStateAdapter) DKG() (protocol.DKG, error) {
	dkg, err := EncodableDKGFromEvents(s.CurrentEpochSetup, s.CurrentEpochCommit)
	if err != nil {
		return nil, fmt.Errorf("could not construct encodable DKG from events: %w", err)
	}

	return NewDKG(dkg), nil
}

// Entry Returns low-level protocol state entry that was used to initialize this object.
// It shouldn't be used by high-level logic, it is useful for some cases such as bootstrapping.
// Prefer using other methods to access protocol state.
func (s *EpochProtocolStateAdapter) Entry() *flow.RichProtocolStateEntry {
	return s.RichProtocolStateEntry.Copy()
}

func (s *EpochProtocolStateAdapter) Identities() flow.IdentityList {
	return s.RichProtocolStateEntry.CurrentEpochIdentityTable
}

func (s *EpochProtocolStateAdapter) GlobalParams() protocol.GlobalParams {
	return s.params
}

// InvalidEpochTransitionAttempted denotes whether an invalid epoch state transition was attempted
// on the fork ending this block. Once the first block where this flag is true is finalized, epoch
// fallback mode is triggered.
// TODO for 'leaving Epoch Fallback via special service event': at the moment, this is a one-way transition and requires a spork to recover - need to revisit for sporkless EFM recovery
func (s *EpochProtocolStateAdapter) InvalidEpochTransitionAttempted() bool {
	return s.ProtocolStateEntry.InvalidEpochTransitionAttempted
}

// PreviousEpochExists returns true if a previous epoch exists. This is true for all epoch
// except those immediately following a spork.
func (s *EpochProtocolStateAdapter) PreviousEpochExists() bool {
	return s.PreviousEpoch != nil
}

// EpochPhase returns the epoch phase for the current epoch.
func (s *EpochProtocolStateAdapter) EpochPhase() flow.EpochPhase {
	return s.Entry().EpochPhase()
}
