package votecollector

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

var (
	// VoteForIncompatibleViewError is emitted, if a view-specific component
	// receives a vote for a different view number.
	VoteForIncompatibleViewError = errors.New("vote for incompatible view")

	// VoteForIncompatibleBlockError is emitted, if a block-specific component
	// receives a vote for a different block ID.
	VoteForIncompatibleBlockError = errors.New("vote for incompatible block")

	// DuplicatedVoteErr is emitted, when we receive an _identical_ duplicated
	// vote for the same block from the same block. This error does _not_
	// indicate equivocation.
	DuplicatedVoteErr = errors.New("duplicated vote")
)

// NoopProcessor implements hotstuff.VoteProcessor. It drops all votes.
type NoopProcessor struct {
	status hotstuff.VoteCollectorStatus
}

func NewNoopCollector(status hotstuff.VoteCollectorStatus) *NoopProcessor {
	return &NoopProcessor{status}
}

func (c *NoopProcessor) Process(*model.Vote) error            { return nil }
func (c *NoopProcessor) Status() hotstuff.VoteCollectorStatus { return c.status }

// ConflictingProposalError indicates that a conflicting block for
// the same view has already been ingested.
type ConflictingProposalError struct {
	ConflictingBlock *model.Block
	err              error
}

func (e ConflictingProposalError) Error() string {
	return e.err.Error()
}

// IsConflictingProposalError returns whether err is a ConflictingProposalError
func IsConflictingProposalError(err error) bool {
	var e ConflictingProposalError
	return errors.As(err, &e)
}

// AsConflictingProposalError determines whether the given error is a ConflictingProposalError
// (potentially wrapped). It follows the same semantics as a checked type cast.
func AsConflictingProposalError(err error) (*ConflictingProposalError, bool) {
	var e ConflictingProposalError
	ok := errors.As(err, &e)
	if ok {
		return &e, true
	}
	return nil, false
}

func (e ConflictingProposalError) Unwrap() error {
	return e.err
}

func NewConflictingProposalErrorf(conflictingBlock *model.Block, msg string, args ...interface{}) error {
	return ConflictingProposalError{
		ConflictingBlock: conflictingBlock,
		err:              fmt.Errorf(msg, args...),
	}
}
