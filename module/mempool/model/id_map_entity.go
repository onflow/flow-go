package model

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// IdMapEntity is an internal data structure for mempool
// It implements a key-value entry where an identifier is mapped to a list of other identifiers.
type IdMapEntity struct {
	Key flow.Identifier
	IDs []flow.Identifier
}

// ID implements flow.Entity.ID for IdMapEntity to make it capable of being stored directly
// in mempools and storage. It returns key field of the id.
func (id IdMapEntity) ID() flow.Identifier {
	return id.Key
}

// CheckSum implements flow.Entity.CheckSum for IdMapEntity to make it capable of being stored directly
// in mempools and storage. It makes the id of the entire IdMapEntity.
func (id IdMapEntity) Checksum() flow.Identifier {
	return flow.MakeID(id)
}
