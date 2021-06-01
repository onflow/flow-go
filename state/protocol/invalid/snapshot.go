package invalid

import (
	"github.com/onflow/flow-go/model/flow"
)

// Snapshot represents a snapshot referencing an invalid block, or for
// which an error occurred while resolving the reference block.
type Snapshot struct {
	err error
}

func NewSnapshot(err error) *Snapshot {
	return &Snapshot{err: err}
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

func (u *Snapshot) Identities(_ flow.IdentityFilter) (flow.IdentityList, error) {
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

func (u *Snapshot) SealingSegment() ([]*flow.Block, error) {
	return nil, u.err
}

func (u *Snapshot) Descendants() ([]flow.Identifier, error) {
	return nil, u.err
}

func (u *Snapshot) ValidDescendants() ([]flow.Identifier, error) {
	return nil, u.err
}

func (u *Snapshot) Seed(_ ...uint32) ([]byte, error) {
	return nil, u.err
}
