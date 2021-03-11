package state

import (
	"github.com/onflow/flow-go/model/flow"
)

type View interface {
	NewChild() View
	MergeView(child View)
	DropDelta() // drops all the delta changes
	Ledger
}

// Rename this to Storage
// remove reference to flow.RegisterValue and use byte[]
// A Ledger is the storage interface used by the virtual machine to read and write register values.
type Ledger interface {
	Set(owner, controller, key string, value flow.RegisterValue) error
	Get(owner, controller, key string) (flow.RegisterValue, error)
	Touch(owner, controller, key string) error
	Delete(owner, controller, key string) error
}
