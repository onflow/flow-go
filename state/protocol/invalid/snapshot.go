package invalid

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
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
	if errors.Is(err, state.ErrUnknownSnapshotReference) {
		return &Snapshot{err: err}
	}
	return &Snapshot{fmt.Errorf("critical unexpected error querying snapshot: %w", err)}
}

// NewSnapshotf is NewSnapshot with ergonomic error formatting.
func NewSnapshotf(msg string, args ...interface{}) *Snapshot {
	return NewSnapshot(fmt.Errorf(msg, args...))
}

func (u *Snapshot) Head() (*flow.Header, error) {
	return nil, u.err
}

func (u *Snapshot) QuorumCertificate() (*flow.QuorumCertificate, error) {
	return nil, u.err
}

func (u *Snapshot) Phase() (flow.EpochPhase, error) {
	return 0, u.err
}

func (u *Snapshot) Identities(_ flow.IdentityFilter[flow.Identity]) (flow.IdentityList, error) {
	return nil, u.err
}

func (u *Snapshot) Identity(_ flow.Identifier) (*flow.Identity, error) {
	return nil, u.err
}

func (u *Snapshot) Commit() (flow.StateCommitment, error) {
	return flow.DummyStateCommitment, u.err
}

func (u *Snapshot) SealedResult() (*flow.ExecutionResult, *flow.Seal, error) {
	return nil, nil, u.err
}

func (u *Snapshot) SealingSegment() (*flow.SealingSegment, error) {
	return nil, u.err
}

func (u *Snapshot) Descendants() ([]flow.Identifier, error) {
	return nil, u.err
}

func (u *Snapshot) RandomSource() ([]byte, error) {
	return nil, u.err
}

func (u *Snapshot) Params() protocol.GlobalParams {
	return Params{u.err}
}

func (u *Snapshot) ProtocolState() (protocol.DynamicProtocolState, error) {
	return nil, u.err
}

func (u *Snapshot) VersionBeacon() (*flow.SealedVersionBeacon, error) {
	return nil, u.err
}
