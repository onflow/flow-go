package model

import (
	"errors"
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

type NoVoteError struct {
	Msg string
}

func (e NoVoteError) Error() string { return e.Msg }

func (e NoVoteError) Is(other error) bool {
	_, ok := other.(NoVoteError)
	return ok
}

var ErrInsufficientVotes = errors.New("received insufficient votes")
var ErrUnverifiableBlock = errors.New("block proposal can't be verified, because its view is above the finalized view, but its QC is below the finalized view")
var ErrInvalidSigner = errors.New("invalid signer(s)")
var ErrInvalidSignature = errors.New("invalid signature")

type ErrorConfiguration struct {
	Msg string
}

func (e *ErrorConfiguration) Error() string { return e.Msg }

type ErrorMissingBlock struct {
	View    uint64
	BlockID flow.Identifier
}

func (e ErrorMissingBlock) Error() string {
	return fmt.Sprintf("missing Block at view %d with ID %v", e.View, e.BlockID)
}

func (e ErrorMissingBlock) Is(other error) bool {
	_, ok := other.(ErrorMissingBlock)
	return ok
}

type ErrorInvalidBlock struct {
	BlockID flow.Identifier
	View    uint64
	Err     error
}

func (e ErrorInvalidBlock) Error() string {
	return fmt.Sprintf("invalid block %x at view %d: %s", e.BlockID, e.View, e.Err.Error())
}

func (e ErrorInvalidBlock) Is(other error) bool {
	_, ok := other.(ErrorInvalidBlock)
	return ok || errors.Is(other, e.Err)
}

type ErrorInvalidVote struct {
	VoteID flow.Identifier
	View   uint64
	Err    error
}

func (e ErrorInvalidVote) Error() string {
	return fmt.Sprintf("invalid vote %x for view %d: %s", e.VoteID, e.View, e.Err.Error())
}

func (e ErrorInvalidVote) Is(other error) bool {
	_, ok := other.(ErrorInvalidVote)
	return ok || errors.Is(other, e.Err)
}

// ErrorByzantineThresholdExceeded is raised if HotStuff detects malicious conditions which
// prove a Byzantine threshold of consensus replicas has been exceeded.
// Per definition, the byzantine threshold is exceeded is there are byzantine consensus
// replicas with _at least_ 1/3 stake.
type ErrorByzantineThresholdExceeded struct {
	Evidence string
}

func (e *ErrorByzantineThresholdExceeded) Error() string {
	return e.Evidence
}
