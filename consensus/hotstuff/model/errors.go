package model

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

var (
	ErrUnverifiableBlock = errors.New("block proposal can't be verified, because its view is above the finalized view, but its QC is below the finalized view")
	ErrInvalidFormat     = errors.New("invalid signature format")
	ErrInvalidSignature  = errors.New("invalid signature")
)

// NoVoteError contains the reason of why the voter didn't vote for a block proposal.
type NoVoteError struct {
	Msg string
}

func (e NoVoteError) Error() string { return e.Msg }

// IsNoVoteError returns whether an err is a NoVoteError
func IsNoVoteError(err error) bool {
	var e NoVoteError
	return errors.As(err, &e)
}

// ConfigurationError indicates that a constructor or component was initialized with
// invalid or inconsistent parameters.
type ConfigurationError struct {
	err error
}

func NewConfigurationError(err error) error {
	return ConfigurationError{err}
}

func NewConfigurationErrorf(msg string, args ...interface{}) error {
	return ConfigurationError{fmt.Errorf(msg, args...)}
}

func (e ConfigurationError) Error() string { return e.err.Error() }
func (e ConfigurationError) Unwrap() error { return e.err }

// IsConfigurationError returns whether err is a ConfigurationError
func IsConfigurationError(err error) bool {
	var e ConfigurationError
	return errors.As(err, &e)
}

// MissingBlockError indicates that no block with identifier `BlockID` is known
type MissingBlockError struct {
	View    uint64
	BlockID flow.Identifier
}

func (e MissingBlockError) Error() string {
	return fmt.Sprintf("missing Block at view %d with ID %v", e.View, e.BlockID)
}

// IsMissingBlockError returns whether err is a MissingBlockError
func IsMissingBlockError(err error) bool {
	var e MissingBlockError
	return errors.As(err, &e)
}

// InvalidBlockError indicates that the block with identifier `BlockID` is invalid
type InvalidBlockError struct {
	BlockID flow.Identifier
	View    uint64
	Err     error
}

func (e InvalidBlockError) Error() string {
	return fmt.Sprintf("invalid block %x at view %d: %s", e.BlockID, e.View, e.Err.Error())
}

// IsInvalidBlockError returns whether err is an InvalidBlockError
func IsInvalidBlockError(err error) bool {
	var e InvalidBlockError
	return errors.As(err, &e)
}

func (e InvalidBlockError) Unwrap() error {
	return e.Err
}

// InvalidVoteError indicates that the vote with identifier `VoteID` is invalid
type InvalidVoteError struct {
	VoteID flow.Identifier
	View   uint64
	Err    error
}

func NewInvalidVoteErrorf(vote *Vote, msg string, args ...interface{}) error {
	return InvalidVoteError{
		VoteID: vote.ID(),
		View:   vote.View,
		Err:    fmt.Errorf(msg, args...),
	}
}

func (e InvalidVoteError) Error() string {
	return fmt.Sprintf("invalid vote %x for view %d: %s", e.VoteID, e.View, e.Err.Error())
}

// IsInvalidVoteError returns whether err is an InvalidVoteError
func IsInvalidVoteError(err error) bool {
	var e InvalidVoteError
	return errors.As(err, &e)
}

func (e InvalidVoteError) Unwrap() error {
	return e.Err
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

// DoubleVoteError indicates that a consensus replica has voted for two _different_
// blocks, or has provided two semantically different votes for the same block.
type DoubleVoteError struct {
	FirstVote       *Vote
	ConflictingVote *Vote
	err             error
}

func (e DoubleVoteError) Error() string {
	return e.err.Error()
}

// IsDoubleVoteError returns whether err is a DoubleVoteError
func IsDoubleVoteError(err error) bool {
	var e DoubleVoteError
	return errors.As(err, &e)
}

// AsDoubleVoteError determines whether err is a DoubleVoteError (potentially
// wrapped). It follows the same semantics as a checked type cast.
func AsDoubleVoteError(err error) (*DoubleVoteError, bool) {
	var e DoubleVoteError
	ok := errors.As(err, &e)
	if ok {
		return &e, true
	}
	return nil, false
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

// DEP_InconsistentVoteError indicates that a consensus replica has emitted
// inconsistent votes for the _same_ block. This is only relevant for voting
// schemes where the voting replica has different options how to sign (e.g. a
// replica can sign with their staking key and/or their random beacon key).
// For such voting schemes, byzantine replicas could try to submit different
// votes for the same block, to exhaust the primary's resources or have
// multiple of their votes counted to undermine consensus safety. Sending
// inconsistent votes belongs to the family of equivocation attacks.
type DEP_InconsistentVoteError struct {
	FirstVote        *Vote
	InconsistentVote *Vote
	err              error
}

func (e DEP_InconsistentVoteError) Error() string {
	return e.err.Error()
}

// DEP_IsInconsistentVoteError returns whether err is an DEP_InconsistentVoteError
func DEP_IsInconsistentVoteError(err error) bool {
	var e DEP_InconsistentVoteError
	return errors.As(err, &e)
}

// DEP_AsInconsistentVoteError determines whether err is a DEP_InconsistentVoteError
// (potentially wrapped). It follows the same semantics as a checked type cast.
func DEP_AsInconsistentVoteError(err error) (*DEP_InconsistentVoteError, bool) {
	var e DEP_InconsistentVoteError
	ok := errors.As(err, &e)
	if ok {
		return &e, true
	}
	return nil, false
}

func (e DEP_InconsistentVoteError) Unwrap() error {
	return e.err
}

func DEP_NewInconsistentVoteErrorf(firstVote, inconsistentVote *Vote, msg string, args ...interface{}) error {
	return DEP_InconsistentVoteError{
		FirstVote:        firstVote,
		InconsistentVote: inconsistentVote,
		err:              fmt.Errorf(msg, args...),
	}
}

// DuplicatedSignerError indicates that a signature from the same node ID has already been added
type DuplicatedSignerError struct {
	err error
}

func NewDuplicatedSignerError(err error) error {
	return DuplicatedSignerError{err}
}

func NewDuplicatedSignerErrorf(msg string, args ...interface{}) error {
	return DuplicatedSignerError{err: fmt.Errorf(msg, args...)}
}

func (e DuplicatedSignerError) Error() string { return e.err.Error() }
func (e DuplicatedSignerError) Unwrap() error { return e.err }

// IsDuplicatedSignerError returns whether err is an DuplicatedSignerError
func IsDuplicatedSignerError(err error) bool {
	var e DuplicatedSignerError
	return errors.As(err, &e)
}

// InvalidSignatureIncludedError indicates that some signatures, included via TrustedAdd, are invalid
type InvalidSignatureIncludedError struct {
	err error
}

func NewInvalidSignatureIncludedError(err error) error {
	return InvalidSignatureIncludedError{err}
}

func NewInvalidSignatureIncludedErrorf(msg string, args ...interface{}) error {
	return InvalidSignatureIncludedError{fmt.Errorf(msg, args...)}
}

func (e InvalidSignatureIncludedError) Error() string { return e.err.Error() }
func (e InvalidSignatureIncludedError) Unwrap() error { return e.err }

// IsInvalidSignatureIncludedError returns whether err is an InvalidSignatureIncludedError
func IsInvalidSignatureIncludedError(err error) bool {
	var e InvalidSignatureIncludedError
	return errors.As(err, &e)
}

// InsufficientSignaturesError indicates that not enough signatures have been stored to complete the operation.
type InsufficientSignaturesError struct {
	err error
}

func NewInsufficientSignaturesError(err error) error {
	return InsufficientSignaturesError{err}
}

func NewInsufficientSignaturesErrorf(msg string, args ...interface{}) error {
	return InsufficientSignaturesError{fmt.Errorf(msg, args...)}
}

func (e InsufficientSignaturesError) Error() string { return e.err.Error() }
func (e InsufficientSignaturesError) Unwrap() error { return e.err }

// IsInsufficientSignaturesError returns whether err is an InsufficientSignaturesError
func IsInsufficientSignaturesError(err error) bool {
	var e InsufficientSignaturesError
	return errors.As(err, &e)
}

// InvalidSignerError indicates that the signer is not authorized or unknown
type InvalidSignerError struct {
	err error
}

func NewInvalidSignerError(err error) error {
	return InvalidSignerError{err}
}

func NewInvalidSignerErrorf(msg string, args ...interface{}) error {
	return InvalidSignerError{fmt.Errorf(msg, args...)}
}

func (e InvalidSignerError) Error() string { return e.err.Error() }
func (e InvalidSignerError) Unwrap() error { return e.err }

// IsInvalidSignerError returns whether err is an InvalidSignerError
func IsInvalidSignerError(err error) bool {
	var e InvalidSignerError
	return errors.As(err, &e)
}

// InconsistentVoteError indicates that a consensus replica has emitted
// inconsistent votes within the same view. We consider two votes as
// inconsistent, if they are from the same signer for the same view, but have
// different IDs. Potential causes could be:
//  * The signer voted for different blocks in the same view.
//  * The signer emitted votes for the same block, but included different
//    signatures. This is only relevant for voting schemes where the signer has
//    different options how to sign (e.g. sign with staking key and/or random
//    beacon key). For such voting schemes, byzantine replicas could try to
//    submit different votes for the same block, to exhaust the primary's
//    resources or have multiple of their votes counted to undermine consensus
//    safety (aka double-counting attack).
// Both are protocol violations and belong to the family of equivocation attacks.
// The InconsistentVoteError does not specify the specific root cause.
type InconsistentVoteError struct {
	InconsistentVote *Vote
	err              error
}

func (e InconsistentVoteError) Error() string { return e.err.Error() }
func (e InconsistentVoteError) Unwrap() error { return e.err }

// IsInconsistentVoteError returns whether err is an InconsistentVoteError
func IsInconsistentVoteError(err error) bool {
	var e InconsistentVoteError
	return errors.As(err, &e)
}

// AsInconsistentVoteError determines whether err is a InconsistentVoteError
// (potentially wrapped). It follows the same semantics as a checked type cast.
func AsInconsistentVoteError(err error) (*InconsistentVoteError, bool) {
	var e InconsistentVoteError
	ok := errors.As(err, &e)
	if ok {
		return &e, true
	}
	return nil, false
}

func NewInconsistentVoteErrorf(inconsistentVote *Vote, msg string, args ...interface{}) error {
	return InconsistentVoteError{
		InconsistentVote: inconsistentVote,
		err:              fmt.Errorf(msg, args...),
	}
}
