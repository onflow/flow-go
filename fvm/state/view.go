package state

import (
	"github.com/onflow/flow-go/model/flow"
)

type View interface {
	NewChild() View
	MergeView(child View) error
	DropDelta() // drops all the delta changes

	// UpdatedRegisters returns all registers that were updated by this view.
	// The returned entries are sorted by ids.
	UpdatedRegisters() flow.RegisterEntries

	// UpdatedRegisterIDs returns all register ids that were updated by this
	// view.  The returned ids are unsorted.
	UpdatedRegisterIDs() []flow.RegisterID

	// AllRegisterIDs returns all register ids that were touched by this view.
	// The returned ids are unsorted.
	AllRegisterIDs() []flow.RegisterID

	Ledger
}

// Ledger is the storage interface used by the virtual machine to read and write register values.
//
// TODO Rename this to Storage
type Ledger interface {
	// TODO(patrick): replace owner/key tuple with RegisterID
	Set(owner, key string, value flow.RegisterValue) error
	Get(owner, key string) (flow.RegisterValue, error)
}
