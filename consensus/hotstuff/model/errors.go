package model

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// NoVoteError contains the reason of why the voter didn't vote for a block proposal.
type NoVoteError struct {
	Msg string
}

func (e NoVoteError) Error() string { return e.Msg }

// IsNoVoteError returns whether an error is NoVoteError
func IsNoVoteError(err error) bool {
	var e NoVoteError
	return errors.As(err, &e)
}

var ErrUnverifiableBlock = errors.New("block proposal can't be verified, because its view is above the finalized view, but its QC is below the finalized view")
var ErrInvalidSigner = errors.New("invalid signer(s)")
var ErrInvalidSignature = errors.New("invalid signature")

type ConfigurationError struct {
	Msg string
}

func (e ConfigurationError) Error() string { return e.Msg }

type MissingBlockError struct {
	View    uint64
	BlockID flow.Identifier
}

func (e MissingBlockError) Error() string {
	return fmt.Sprintf("missing Block at view %d with ID %v", e.View, e.BlockID)
}

// IsMissingBlockError returns whether an error is MissingBlockError
func IsMissingBlockError(err error) bool {
	var e MissingBlockError
	return errors.As(err, &e)
}

type InvalidBlockError struct {
	BlockID flow.Identifier
	View    uint64
	Err     error
}

func (e InvalidBlockError) Error() string {
	return fmt.Sprintf("invalid block %x at view %d: %s", e.BlockID, e.View, e.Err.Error())
}

// IsInvalidBlockError returns whether an error is InvalidBlockError
func IsInvalidBlockError(err error) bool {
	var e InvalidBlockError
	return errors.As(err, &e)
}

func (e InvalidBlockError) Unwrap() error {
	return e.Err
}

type InvalidVoteError struct {
	VoteID flow.Identifier
	View   uint64
	Err    error
}

func (e InvalidVoteError) Error() string {
	return fmt.Sprintf("invalid vote %x for view %d: %s", e.VoteID, e.View, e.Err.Error())
}

// IsInvalidVoteError returns whether an error is InvalidVoteError
func IsInvalidVoteError(err error) bool {
	var e InvalidVoteError
	return errors.As(err, &e)
}

func (e InvalidVoteError) Unwrap() error {
	return e.Err
}

func NewInvalidVoteErrorf(vote *Vote, msg string, args ...interface{}) error {
	return InvalidVoteError{
		VoteID: vote.ID(),
		View:   vote.View,
		Err:    fmt.Errorf(msg, args...),
	}
}

// ByzantineThresholdExceededError is raised if HotStuff detects malicious conditions which
// prove a Byzantine threshold of consensus replicas has been exceeded.
// Per definition, the byzantine threshold is exceeded is there are byzantine consensus
// replicas with _at least_ 1/3 stake.
type ByzantineThresholdExceededError struct {
	Evidence string
}

func (e ByzantineThresholdExceededError) Error() string {
	return e.Evidence
}

type DoubleVoteError struct {
	FirstVote       *Vote
	ConflictingVote *Vote
	err             error
}

func (e DoubleVoteError) Error() string {
	return e.err.Error()
}

// IsDoubleVoteError returns whether an error is DoubleVoteError
func IsDoubleVoteError(err error) bool {
	var e DoubleVoteError
	return errors.As(err, &e)
}

func (e DoubleVoteError) Unwrap() error {
	return e.err
}

func NewDoubleVoteErrorf(firstVote, conflictingVote *Vote, msg string, args ...interface{}) error {
	return DoubleVoteError{
		FirstVote:       firstVote,
		ConflictingVote: conflictingVote,
		err:             fmt.Errorf(msg, args...),
	}
}
