package state

import (
	"github.com/onflow/flow-go/model/flow"
)

type View interface {
	NewChild() View
	MergeView(child View) error
	DropDelta() // drops all the delta changes
	RegisterUpdates() ([]flow.RegisterID, []flow.RegisterValue)
	AllRegisters() []flow.RegisterID
	Ledger
}

// Ledger is the storage interface used by the virtual machine to read and write register values.
//
// TODO Rename this to Storage
// and remove reference to flow.RegisterValue and use byte[]
type Ledger interface {
	Set(owner, controller, key string, value flow.RegisterValue) error
	Get(owner, controller, key string) (flow.RegisterValue, error)
	Touch(owner, controller, key string) error
	Delete(owner, controller, key string) error
}
