package hotstuff

import (
	"errors"
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

type ErrorFinalizationFatal struct {
	Msg string
}

func (e *ErrorFinalizationFatal) Error() string { return e.Msg }

type MissingSignerError struct {
	Vote *Vote
}

type InvalidSignatureError struct {
	Vote *Vote
}

type InvalidViewError struct {
	Vote *Vote
}

type DoubleVoteError struct {
	OriginalVote *Vote
	DoubleVote   *Vote
}

type StaleVoteError struct {
	Vote              *Vote
	HighestPrunedView uint64
}

type StaleBlockError struct {
	Block             *Block
	HighestPrunedView uint64
}

type ExistingQCError struct {
	Vote *Vote
	QC   *QuorumCertificate
}

var ErrInsufficientVotes = errors.New("received insufficient votes")

func (e MissingSignerError) Error() string {
	return fmt.Sprintf("missing signer of vote %v", e.Vote)
}

func (e InvalidSignatureError) Error() string {
	return fmt.Sprintf("invalid signature for vote %v", e.Vote)
}

func (e InvalidViewError) Error() string {
	return fmt.Sprintf("invalid view for vote %v", e.Vote)
}

func (e DoubleVoteError) Error() string {
	return fmt.Sprintf("double voting detected (original vote: %v, double vote: %v)", e.OriginalVote, e.DoubleVote)
}

func (e StaleVoteError) Error() string {
	return fmt.Sprintf("stale vote (highest pruned view %d): %v", e.HighestPrunedView, e.Vote)
}

func (e StaleBlockError) Error() string {
	return fmt.Sprintf("stale block (highest pruned view %d): %v", e.HighestPrunedView, e.Block)
}

func (e ExistingQCError) Error() string {
	return fmt.Sprintf("QC already existed (vote: %v, qc: %v)", e.Vote, e.QC)
}

type ErrorConfiguration struct {
	Msg string
}

func (e *ErrorConfiguration) Error() string { return e.Msg }

type ErrorConflictingQCs struct {
	View uint64
	QCs  []*QuorumCertificate
}

func (e *ErrorConflictingQCs) Error() string {
	return fmt.Sprintf("%d conflicting QCs at view %d", len(e.QCs), e.View)
}

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
