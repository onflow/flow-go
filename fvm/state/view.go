package state

import (
	"golang.org/x/exp/slices"

	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/model/flow"
)

type View interface {
	NewChild() View

	Finalize() *ExecutionSnapshot
	Merge(child *ExecutionSnapshot) error

	Storage
}

// Storage is the storage interface used by the virtual machine to read and
// write register values.
type Storage interface {
	// TODO(patrick): remove once fvm.VM.Run() is deprecated
	Peek(id flow.RegisterID) (flow.RegisterValue, error)

	Set(id flow.RegisterID, value flow.RegisterValue) error
	Get(id flow.RegisterID) (flow.RegisterValue, error)

	DropChanges() error
}

type ExecutionSnapshot struct {
	// Note that the ReadSet only include reads from the storage snapshot.
	// Reads from the WriteSet are excluded from the ReadSet.
	ReadSet map[flow.RegisterID]struct{}

	WriteSet map[flow.RegisterID]flow.RegisterValue

	// Note that the spock secret may be nil if the view does not support spock.
	SpockSecret []byte

	// Note that the meter may be nil if the view does not support metering.
	*meter.Meter
}

// UpdatedRegisters returns all registers that were updated by this view.
// The returned entries are sorted by ids.
func (snapshot *ExecutionSnapshot) UpdatedRegisters() flow.RegisterEntries {
	entries := make(flow.RegisterEntries, 0, len(snapshot.WriteSet))
	for key, value := range snapshot.WriteSet {
		entries = append(entries, flow.RegisterEntry{Key: key, Value: value})
	}

	slices.SortFunc(entries, func(a, b flow.RegisterEntry) bool {
		return (a.Key.Owner < b.Key.Owner) ||
			(a.Key.Owner == b.Key.Owner && a.Key.Key < b.Key.Key)
	})

	return entries
}

// UpdatedRegisterIDs returns all register ids that were updated by this
// view.  The returned ids are unsorted.
func (snapshot *ExecutionSnapshot) UpdatedRegisterIDs() []flow.RegisterID {
	ids := make([]flow.RegisterID, 0, len(snapshot.WriteSet))
	for key := range snapshot.WriteSet {
		ids = append(ids, key)
	}
	return ids
}

// AllRegisterIDs returns all register ids that were read / write by this
// view. The returned ids are unsorted.
func (snapshot *ExecutionSnapshot) AllRegisterIDs() []flow.RegisterID {
	set := make(
		map[flow.RegisterID]struct{},
		len(snapshot.ReadSet)+len(snapshot.WriteSet))
	for reg := range snapshot.ReadSet {
		set[reg] = struct{}{}
	}
	for reg := range snapshot.WriteSet {
		set[reg] = struct{}{}
	}
	ret := make([]flow.RegisterID, 0, len(set))
	for r := range set {
		ret = append(ret, r)
	}
	return ret
}
