package payload

import (
	"github.com/onflow/flow-go/model/flow"
)

// Storage provides persistant storage backbone for storing historic payload values
type Storage interface {
	Commit(header *flow.Header, update map[flow.RegisterID]flow.RegisterValue) error
	RegisterValueAt(height uint64, id flow.RegisterID) (value flow.RegisterValue, err error)
	LastCommittedBlock() (flow.Identifier, *flow.Header, error)
}
