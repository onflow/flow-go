package payload

import (
	"github.com/onflow/flow-go/model/flow"
)

// Storage provides persistant storage backbone for payload store
type Storage interface {
	Commit(header *flow.Header, update map[flow.RegisterID]flow.RegisterValue) error
	RegisterAt(height uint64, id flow.RegisterID) (value flow.RegisterValue, err error)
	LastCommittedBlockHeight() (uint64, error)
	LastCommittedBlockID() (flow.Identifier, error)
}
