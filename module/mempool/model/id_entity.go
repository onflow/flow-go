package model

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// IdEntity is an internal data structure for mempool
// It implements a wrapper around the original flow Identifier
// which represents it as a flow Entity
// that allows the identifier to directly gets stored in the mempool
type IdEntity struct {
	Id flow.Identifier
}

// ID implements flow.Entity.ID for Identifier to make it capable of being stored directly
// in mempools and storage
// ID returns the identifier itself
func (id IdEntity) ID() flow.Identifier {
	return id.Id
}

// ID implements flow.Entity.ID for Identifier to make it capable of being stored directly
// in mempools and storage
// ID returns checksum of identifier
func (id IdEntity) Checksum() flow.Identifier {
	return flow.MakeID(id)
}
