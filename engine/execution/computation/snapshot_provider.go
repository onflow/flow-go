package computation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// ProtocolSnapshotExecutionSubset is a subset of the protocol state snapshot that is needed by the FVM
var _ flow.ProtocolSnapshotExecutionSubset = (protocol.Snapshot)(nil)

// protocolStateWrapper just wraps the protocol.State and returns a ProtocolSnapshotExecutionSubset
// from the AtBlockID method instead of the protocol.Snapshot interface.
type protocolStateWrapper struct {
	protocol.State
}

// protocolStateWrapper implements `EntropyProviderPerBlock`
var _ flow.ProtocolSnapshotExecutionSubsetProvider = (*protocolStateWrapper)(nil)

func (p protocolStateWrapper) AtBlockID(blockID flow.Identifier) flow.ProtocolSnapshotExecutionSubset {
	return p.State.AtBlockID(blockID)
}

// NewProtocolStateWrapper wraps the protocol.State input so that the AtBlockID method returns a
// ProtocolSnapshotExecutionSubset instead of the protocol.Snapshot interface.
// This is used in the FVM for execution.
func NewProtocolStateWrapper(s protocol.State) flow.ProtocolSnapshotExecutionSubsetProvider {
	return protocolStateWrapper{s}
}
