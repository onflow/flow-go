package events

import (
	"github.com/onflow/flow-go/model"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

// SyncedBlock is an internal message passed from the chainsync engine to the consensus message hub.
// It contains a block obtained through the chainsync protocol.
// These blocks have not been validated and should be considered untrusted inputs,
// however they are guaranteed to be structurally valid: StructureValid()==nil
type SyncedBlock struct {
	OriginID flow.Identifier
	Block    *flow.Block
}

var _ model.StructureValidator = (*SyncedBlock)(nil)

// StructureValid checks basic structural validity of the message and ensures no required fields are nil.
func (m *SyncedBlock) StructureValid() error {
	if m.Block.Header == nil {
		return model.NewStructureInvalidError("nil block header")
	}
	if m.Block.Payload == nil {
		return model.NewStructureInvalidError("nil block payload")
	}
	return nil
}

// SyncedClusterBlock is an internal message passed from the chainsync engine to the consensus message hub.
// It contains a block obtained through the chainsync protocol.
// These blocks have not been validated and should be considered untrusted inputs,
// however they are guaranteed to be structurally valid: StructureValid()==nil
type SyncedClusterBlock struct {
	OriginID flow.Identifier
	Block    *cluster.Block
}

var _ model.StructureValidator = (*SyncedClusterBlock)(nil)

// StructureValid checks basic structural validity of the message and ensures no required fields are nil.
func (m *SyncedClusterBlock) StructureValid() error {
	if m.Block.Header == nil {
		return model.NewStructureInvalidError("nil block header")
	}
	if m.Block.Payload == nil {
		return model.NewStructureInvalidError("nil block payload")
	}
	return nil
}
