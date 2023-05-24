package model

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

var (
	ErrUnverifiableBlock = errors.New("block proposal can't be verified, because its view is above the finalized view, but its QC is below the finalized view")
	ErrInvalidSignature  = errors.New("invalid signature")
	// ErrViewForUnknownEpoch is returned when Epoch information is queried for a view that is
	// outside of all cached epochs. This can happen when a query is made for a view in the
	// next epoch, if that epoch is not committed yet. This can also happen when an
	// old epoch is queried (>3 in the past), even if that epoch does exist in storage.
	ErrViewForUnknownEpoch = fmt.Errorf("by-view query for unknown epoch")
)

// NoVoteError contains the reason why hotstuff.SafetyRules refused to generate a `Vote` for the current view.
type NoVoteError struct {
	Err error
}

func (e NoVoteError) Error() string { return fmt.Sprintf("not voting - %s", e.Err.Error()) }

func (e NoVoteError) Unwrap() error {
	return e.Err
}

// IsNoVoteError returns whether an error is NoVoteError
func IsNoVoteError(err error) bool {
	var e NoVoteError
	return errors.As(err, &e)
}

func NewNoVoteErrorf(msg string, args ...interface{}) error {
	return NoVoteError{Err: fmt.Errorf(msg, args...)}
}

// NoTimeoutError contains the reason why hotstuff.SafetyRules refused to generate a `TimeoutObject` [TO] for the current view.
type NoTimeoutError struct {
	Err error
}

func (e NoTimeoutError) Error() string {
	return fmt.Sprintf("conditions not satisfied to generate valid TimeoutObject: %s", e.Err.Error())
}

func (e NoTimeoutError) Unwrap() error {
	return e.Err
}

// IsNoTimeoutError returns whether an error is NoTimeoutError
func IsNoTimeoutError(err error) bool {
	var e NoTimeoutError
	return errors.As(err, &e)
}

func NewNoTimeoutErrorf(msg string, args ...interface{}) error {
	return NoTimeoutError{Err: fmt.Errorf(msg, args...)}
}

// InvalidFormatError indicates that some data has an incompatible format.
type InvalidFormatError struct {
	err error
}

func NewInvalidFormatError(err error) error {
	return InvalidFormatError{err}
}

func NewInvalidFormatErrorf(msg string, args ...interface{}) error {
	return InvalidFormatError{fmt.Errorf(msg, args...)}
}

func (e InvalidFormatError) Error() string { return e.err.Error() }
func (e InvalidFormatError) Unwrap() error { return e.err }

// IsInvalidFormatError returns whether err is a InvalidFormatError
func IsInvalidFormatError(err error) bool {
	var e InvalidFormatError
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
	return fmt.Sprintf("missing Proposal at view %d with ID %v", e.View, e.BlockID)
}

// IsMissingBlockError returns whether an error is MissingBlockError
func IsMissingBlockError(err error) bool {
	var e MissingBlockError
	return errors.As(err, &e)
}

// InvalidQCError indicates that the QC for block identified by `BlockID` and `View` is invalid
type InvalidQCError struct {
	BlockID flow.Identifier
	View    uint64
	Err     error
}

func (e InvalidQCError) Error() string {
	return fmt.Sprintf("invalid QC for block %x at view %d: %s", e.BlockID, e.View, e.Err.Error())
}

// IsInvalidQCError returns whether an error is InvalidQCError
func IsInvalidQCError(err error) bool {
	var e InvalidQCError
	return errors.As(err, &e)
}

func (e InvalidQCError) Unwrap() error {
	return e.Err
}

// InvalidTCError indicates that the TC for view identified by `View` is invalid
type InvalidTCError struct {
	View uint64
	Err  error
}

func (e InvalidTCError) Error() string {
	return fmt.Sprintf("invalid TC at view %d: %s", e.View, e.Err.Error())
}

// IsInvalidTCError returns whether an error is InvalidQCError
func IsInvalidTCError(err error) bool {
	var e InvalidTCError
	return errors.As(err, &e)
}

func (e InvalidTCError) Unwrap() error {
	return e.Err
}

// InvalidProposalError indicates that the proposal is invalid
type InvalidProposalError struct {
	InvalidProposal *Proposal
	Err             error
}

func NewInvalidProposalErrorf(proposal *Proposal, msg string, args ...interface{}) error {
	return InvalidProposalError{
		InvalidProposal: proposal,
		Err:             fmt.Errorf(msg, args...),
	}
}

func (e InvalidProposalError) Error() string {
	return fmt.Sprintf(
		"invalid proposal %x at view %d: %s",
		e.InvalidProposal.Block.BlockID,
		e.InvalidProposal.Block.View,
		e.Err.Error(),
	)
}

func (e InvalidProposalError) Unwrap() error {
	return e.Err
}

// IsInvalidProposalError returns whether an error is InvalidProposalError
func IsInvalidProposalError(err error) bool {
	var e InvalidProposalError
	return errors.As(err, &e)
}

// AsInvalidProposalError determines whether the given error is a InvalidProposalError
// (potentially wrapped). It follows the same semantics as a checked type cast.
func AsInvalidProposalError(err error) (*InvalidProposalError, bool) {
	var e InvalidProposalError
	ok := errors.As(err, &e)
	if ok {
		return &e, true
	}
	return nil, false
}

// InvalidBlockError indicates that the block is invalid
type InvalidBlockError struct {
	InvalidBlock *Block
	Err          error
}

func NewInvalidBlockErrorf(block *Block, msg string, args ...interface{}) error {
	return InvalidBlockError{
		InvalidBlock: block,
		Err:          fmt.Errorf(msg, args...),
	}
}

func (e InvalidBlockError) Error() string {
	return fmt.Sprintf(
		"invalid block %x at view %d: %s",
		e.InvalidBlock.BlockID,
		e.InvalidBlock.View,
		e.Err.Error(),
	)
}

// IsInvalidBlockError returns whether an error is InvalidBlockError
func IsInvalidBlockError(err error) bool {
	var e InvalidBlockError
	return errors.As(err, &e)
}

// AsInvalidBlockError determines whether the given error is a InvalidProposalError
// (potentially wrapped). It follows the same semantics as a checked type cast.
func AsInvalidBlockError(err error) (*InvalidBlockError, bool) {
	var e InvalidBlockError
	ok := errors.As(err, &e)
	if ok {
		return &e, true
	}
	return nil, false
}

func (e InvalidBlockError) Unwrap() error {
	return e.Err
}

// InvalidVoteError indicates that the vote with identifier `VoteID` is invalid
type InvalidVoteError struct {
	Vote *Vote
	Err  error
}

func NewInvalidVoteErrorf(vote *Vote, msg string, args ...interface{}) error {
	return InvalidVoteError{
		Vote: vote,
		Err:  fmt.Errorf(msg, args...),
	}
}

func (e InvalidVoteError) Error() string {
	return fmt.Sprintf("invalid vote at view %d for block %x: %s", e.Vote.View, e.Vote.BlockID, e.Err.Error())
}

// IsInvalidVoteError returns whether an error is InvalidVoteError
func IsInvalidVoteError(err error) bool {
	var e InvalidVoteError
	return errors.As(err, &e)
}

// AsInvalidVoteError determines whether the given error is a InvalidVoteError
// (potentially wrapped). It follows the same semantics as a checked type cast.
func AsInvalidVoteError(err error) (*InvalidVoteError, bool) {
	var e InvalidVoteError
	ok := errors.As(err, &e)
	if ok {
		return &e, true
	}
	return nil, false
}

func (e InvalidVoteError) Unwrap() error {
	return e.Err
}

// ByzantineThresholdExceededError is raised if HotStuff detects malicious conditions, which
// prove that the Byzantine threshold of consensus replicas has been exceeded. Per definition,
// this is the case when there are byzantine consensus replicas with ≥ 1/3 of the committee's
// total weight. In this scenario, foundational consensus safety guarantees fail.
// Generally, the protocol cannot continue in such conditions.
// We represent this exception as with a dedicated type, so its occurrence can be detected by
// higher-level logic and escalated to the node operator.
type ByzantineThresholdExceededError struct {
	Evidence string
}

func (e ByzantineThresholdExceededError) Error() string {
	return e.Evidence
}

func IsByzantineThresholdExceededError(err error) bool {
	var target ByzantineThresholdExceededError
	return errors.As(err, &target)
}

// DoubleVoteError indicates that a consensus replica has voted for two different
// blocks, or has provided two semantically different votes for the same block.
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

// AsDoubleVoteError determines whether the given error is a DoubleVoteError
// (potentially wrapped). It follows the same semantics as a checked type cast.
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

// InvalidAggregatedKeyError indicates that the aggregated key is invalid
// which makes any aggregated signature invalid.
type InvalidAggregatedKeyError struct {
	error
}

func NewInvalidAggregatedKeyError(err error) error {
	return InvalidAggregatedKeyError{err}
}

func NewInvalidAggregatedKeyErrorf(msg string, args ...interface{}) error {
	return InvalidAggregatedKeyError{fmt.Errorf(msg, args...)}
}

func (e InvalidAggregatedKeyError) Unwrap() error { return e.error }

// IsInvalidAggregatedKeyError returns whether err is an InvalidAggregatedKeyError
func IsInvalidAggregatedKeyError(err error) bool {
	var e InvalidAggregatedKeyError
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

// DoubleTimeoutError indicates that a consensus replica has created two different
// timeout objects for same view.
type DoubleTimeoutError struct {
	FirstTimeout       *TimeoutObject
	ConflictingTimeout *TimeoutObject
	err                error
}

func (e DoubleTimeoutError) Error() string {
	return e.err.Error()
}

// IsDoubleTimeoutError returns whether an error is DoubleTimeoutError
func IsDoubleTimeoutError(err error) bool {
	var e DoubleTimeoutError
	return errors.As(err, &e)
}

// AsDoubleTimeoutError determines whether the given error is a DoubleTimeoutError
// (potentially wrapped). It follows the same semantics as a checked type cast.
func AsDoubleTimeoutError(err error) (*DoubleTimeoutError, bool) {
	var e DoubleTimeoutError
	ok := errors.As(err, &e)
	if ok {
		return &e, true
	}
	return nil, false
}

func (e DoubleTimeoutError) Unwrap() error {
	return e.err
}

func NewDoubleTimeoutErrorf(firstTimeout, conflictingTimeout *TimeoutObject, msg string, args ...interface{}) error {
	return DoubleTimeoutError{
		FirstTimeout:       firstTimeout,
		ConflictingTimeout: conflictingTimeout,
		err:                fmt.Errorf(msg, args...),
	}
}

// InvalidTimeoutError indicates that the embedded timeout object is invalid
type InvalidTimeoutError struct {
	Timeout *TimeoutObject
	Err     error
}

func NewInvalidTimeoutErrorf(timeout *TimeoutObject, msg string, args ...interface{}) error {
	return InvalidTimeoutError{
		Timeout: timeout,
		Err:     fmt.Errorf(msg, args...),
	}
}

func (e InvalidTimeoutError) Error() string {
	return fmt.Sprintf("invalid timeout %x for view %d: %s", e.Timeout.ID(), e.Timeout.View, e.Err.Error())
}

// IsInvalidTimeoutError returns whether an error is InvalidTimeoutError
func IsInvalidTimeoutError(err error) bool {
	var e InvalidTimeoutError
	return errors.As(err, &e)
}

// AsInvalidTimeoutError determines whether the given error is a InvalidTimeoutError
// (potentially wrapped). It follows the same semantics as a checked type cast.
func AsInvalidTimeoutError(err error) (*InvalidTimeoutError, bool) {
	var e InvalidTimeoutError
	ok := errors.As(err, &e)
	if ok {
		return &e, true
	}
	return nil, false
}

func (e InvalidTimeoutError) Unwrap() error {
	return e.Err
}
