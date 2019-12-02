package tests

import (
	"github.com/dapperlabs/flow-go/model/collection"
	"github.com/dapperlabs/flow-go/network/stub"
)

// blocking this event will cause propagation engine's snapshot to be in sync with its peer.
func blockGuaranteedCollection(m *stub.PendingMessage) bool {
	switch m.Event.(type) {
	case *collection.GuaranteedCollection:
		return true
	default:
		return false
	}
}
