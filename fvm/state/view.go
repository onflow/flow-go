package state

import (
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/model/flow"
)

type View interface {
	NewChild() View

	Merge(child ExecutionSnapshot) error

	ExecutionSnapshot

	Storage
}

// Storage is the storage interface used by the virtual machine to read and
// write register values.
type Storage interface {
	Set(id flow.RegisterID, value flow.RegisterValue) error
	Get(id flow.RegisterID) (flow.RegisterValue, error)

	DropChanges() error
}

type ExecutionSnapshot interface {
	// UpdatedRegisters returns all registers that were updated by this view.
	// The returned entries are sorted by ids.
	UpdatedRegisters() flow.RegisterEntries

	// UpdatedRegisterIDs returns all register ids that were updated by this
	// view.  The returned ids are unsorted.
	UpdatedRegisterIDs() []flow.RegisterID

	// AllRegisterIDs returns all register ids that were read / write by this
	// view. The returned ids are unsorted.
	AllRegisterIDs() []flow.RegisterID

	// TODO(patrick): implement this.
	//
	// StorageSnapshotRegisterIDs returns all register ids that were read
	// from the underlying storage snapshot / view. The returned ids are
	// unsorted.
	// StorageSnapshotRegisterIDs() []flow.RegisterID

	// Note that the returned spock secret may be nil if the view does not
	// support spock.
	SpockSecret() []byte

	// Note that the returned meter may be nil if the view does not
	// support metering.
	Meter() *meter.Meter
}
