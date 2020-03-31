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

type ErrorConfiguration struct {
	Msg string
}

func (e *ErrorConfiguration) Error() string { return e.Msg }

type ErrorMissingBlock struct {
	View    uint64
	BlockID flow.Identifier
}

func (e *ErrorMissingBlock) Error() string {
	return fmt.Sprintf("missing Block at view %d with ID %v", e.View, e.BlockID)
}

type ErrorInvalidBlock struct {
	View    uint64
	BlockID flow.Identifier
	Msg     string
}

func (e ErrorInvalidBlock) Error() string {
	return fmt.Sprintf("invalid block (view %d; ID %x): %s", e.View, e.BlockID, e.Msg)
}

func (e ErrorInvalidBlock) Is(other error) bool {
	_, ok := other.(ErrorInvalidBlock)
	return ok
}

type ErrorInvalidVote struct {
	VoteID flow.Identifier
	View   uint64
	Msg    string
}

func (e ErrorInvalidVote) Error() string {
	return fmt.Sprintf("invalid vote (view %d; ID %x): %s", e.View, e.VoteID, e.Msg)
}

func (e ErrorInvalidVote) Is(other error) bool {
	_, ok := other.(ErrorInvalidVote)
	return ok
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
