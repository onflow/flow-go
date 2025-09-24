package invalid

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/cluster"
)

// Snapshot represents a snapshot that does not exist or could not be queried.
type Snapshot struct {
	err error
}

// NewSnapshot returns a new invalid snapshot, containing an error describing why the
// snapshot could not be retrieved. The following are expected
// errors when constructing an invalid Snapshot:
//   - state.ErrUnknownSnapshotReference if the reference point for the snapshot
//     (height or block ID) does not resolve to a queriable block in the state.
//   - generic error in case of unexpected critical internal corruption or bugs
func NewSnapshot(err error) *Snapshot {
	if err == nil {
		return &Snapshot{err: fmt.Errorf("invalid snapshot: no error provided")}
	}
	return &Snapshot{err: err}
}

var _ cluster.Snapshot = (*Snapshot)(nil)

// NewSnapshotf is NewSnapshot with ergonomic error formatting.
func NewSnapshotf(msg string, args ...interface{}) *Snapshot {
	return NewSnapshot(fmt.Errorf(msg, args...))
}

func (u *Snapshot) Collection() (*flow.Collection, error) {
	return nil, u.err
}

func (u *Snapshot) Head() (*flow.Header, error) {
	return nil, u.err
}

func (u *Snapshot) Pending() ([]flow.Identifier, error) {
	return nil, u.err
}
