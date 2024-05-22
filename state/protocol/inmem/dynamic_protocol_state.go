package inmem

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// DynamicProtocolStateAdapter implements protocol.DynamicProtocolState by wrapping an InitialProtocolStateAdapter.
type DynamicProtocolStateAdapter struct {
	InitialProtocolStateAdapter // TODO replace with RichProtocolStateEntry
	params                      protocol.GlobalParams
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

func (s *DynamicProtocolStateAdapter) Epoch() uint64 {
	return s.CurrentEpochSetup.Counter
}

func (s *DynamicProtocolStateAdapter) Clustering() (flow.ClusterList, error) {
	clustering, err := ClusteringFromSetupEvent(s.CurrentEpochSetup)
	if err != nil {
		return nil, fmt.Errorf("could not extract cluster list from setup event: %w", err)
	}
	return clustering, nil
}

func (s *DynamicProtocolStateAdapter) EpochSetup() *flow.EpochSetup {
	return s.CurrentEpochSetup
}

func (s *DynamicProtocolStateAdapter) EpochCommit() *flow.EpochCommit {
	return s.CurrentEpochCommit
}

func (s *DynamicProtocolStateAdapter) DKG() (protocol.DKG, error) {
	dkg, err := EncodableDKGFromEvents(s.CurrentEpochSetup, s.CurrentEpochCommit)
	if err != nil {
		return nil, fmt.Errorf("could not construct encodable DKG from events: %w", err)
	}

	return NewDKG(dkg), nil
}

// Entry Returns low-level protocol state entry that was used to initialize this object.
// It shouldn't be used by high-level logic, it is useful for some cases such as bootstrapping.
// Prefer using other methods to access protocol state.
func (s *DynamicProtocolStateAdapter) Entry() *flow.RichProtocolStateEntry {
	return s.RichProtocolStateEntry.Copy()
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
