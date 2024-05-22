package inmem

import (
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
