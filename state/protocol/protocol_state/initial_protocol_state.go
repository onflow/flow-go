package protocol_state

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
)

// initialProtocolStateAdapter implements protocol.InitialProtocolState by wrapping a RichProtocolStateEntry.
// TODO(yuraolex): for sake of avoiding errors in return values in interface methods this adapter pre-caches
// some values. This is debatable as clustering for instance is not accessed frequently and could be lazily loaded.
// The problem with lazy loading is handling error value from `inmem.ClusteringFromSetupEvent`. There are two ways to avoid it:
// 1. Return error from interface method.
// 2. Inject irrecoverable.Signaler into the adapter and panic on error since any error in that method has to be a severe implementation bug.
type initialProtocolStateAdapter struct {
	*flow.RichProtocolStateEntry
	clustering flow.ClusterList
	dkg        protocol.DKG
}

var _ protocol.InitialProtocolState = (*initialProtocolStateAdapter)(nil)

func newInitialProtocolStateAdapter(entry *flow.RichProtocolStateEntry) (*initialProtocolStateAdapter, error) {
	dkg, err := inmem.EncodableDKGFromEvents(entry.CurrentEpochSetup, entry.CurrentEpochCommit)
	if err != nil {
		return nil, fmt.Errorf("could not construct encodable DKG from events: %w", err)
	}

	clustering, err := inmem.ClusteringFromSetupEvent(entry.CurrentEpochSetup)
	if err != nil {
		return nil, fmt.Errorf("could not extract cluster list from setup event: %w", err)
	}

	return &initialProtocolStateAdapter{
		RichProtocolStateEntry: entry,
		clustering:             clustering,
		dkg:                    inmem.NewDKG(dkg),
	}, nil
}

func (s *initialProtocolStateAdapter) Epoch() uint64 {
	return s.CurrentEpochSetup.Counter
}

func (s *initialProtocolStateAdapter) Clustering() flow.ClusterList {
	return s.clustering
}

func (s *initialProtocolStateAdapter) EpochSetup() *flow.EpochSetup {
	return s.CurrentEpochSetup
}

func (s *initialProtocolStateAdapter) EpochCommit() *flow.EpochCommit {
	return s.CurrentEpochCommit
}

func (s *initialProtocolStateAdapter) DKG() protocol.DKG {
	return s.dkg
}
