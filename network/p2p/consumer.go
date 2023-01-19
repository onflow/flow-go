package p2p

import (
	"github.com/onflow/flow-go/model/flow"
)

// NodeBlockListConsumer consumes notifications from the cache.NodeBlocklistWrapper whenever the block list is updated.
// Implementations must:
//   - be concurrency safe
//   - be non-blocking
type NodeBlockListConsumer interface {
	// OnNodeBlockListUpdate notifications whenever the node block list is updated.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnNodeBlockListUpdate(list flow.IdentifierList)
}
