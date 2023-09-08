package inmem

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// InitialProtocolStateAdapter implements protocol.InitialProtocolState by wrapping a RichProtocolStateEntry.
// TODO(yuraolex): for sake of avoiding errors in return values in interface methods this adapter pre-caches
// some values. This is debatable as clustering for instance is not accessed frequently and could be lazily loaded.
// The problem with lazy loading is handling error value from `inmem.ClusteringFromSetupEvent`. There are two ways to avoid it:
// 1. Return error from interface method.
// 2. Inject irrecoverable.Signaler into the adapter and panic on error since any error in that method has to be a severe implementation bug.
type InitialProtocolStateAdapter struct {
	*flow.RichProtocolStateEntry
}

var _ protocol.InitialProtocolState = (*InitialProtocolStateAdapter)(nil)

func NewInitialProtocolStateAdapter(entry *flow.RichProtocolStateEntry) *InitialProtocolStateAdapter {
	return &InitialProtocolStateAdapter{
		RichProtocolStateEntry: entry,
	}
}

func (s *InitialProtocolStateAdapter) Epoch() uint64 {
	return s.CurrentEpochSetup.Counter
}

func (s *InitialProtocolStateAdapter) Clustering() (flow.ClusterList, error) {
	clustering, err := ClusteringFromSetupEvent(s.CurrentEpochSetup)
	if err != nil {
		return nil, fmt.Errorf("could not extract cluster list from setup event: %w", err)
	}
	return clustering, nil
}

func (s *InitialProtocolStateAdapter) EpochSetup() *flow.EpochSetup {
	return s.CurrentEpochSetup
}

func (s *InitialProtocolStateAdapter) EpochCommit() *flow.EpochCommit {
	return s.CurrentEpochCommit
}

func (s *InitialProtocolStateAdapter) DKG() (protocol.DKG, error) {
	dkg, err := EncodableDKGFromEvents(s.CurrentEpochSetup, s.CurrentEpochCommit)
	if err != nil {
		return nil, fmt.Errorf("could not construct encodable DKG from events: %w", err)
	}

	return NewDKG(dkg), nil
}

// Entry Returns low-level protocol state entry that was used to initialize this object.
// It shouldn't be used by high-level logic, it is useful for some cases such as bootstrapping.
// Prefer using other methods to access protocol state.
func (s *InitialProtocolStateAdapter) Entry() *flow.RichProtocolStateEntry {
	return s.RichProtocolStateEntry.Copy()
}
