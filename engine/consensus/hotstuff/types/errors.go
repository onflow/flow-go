package types

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

type ErrorFinalizationFatal struct {
	Msg string
}

func (e *ErrorFinalizationFatal) Error() string { return e.Msg }

type ErrorMissingSigner struct {
	Vote *Vote
}

type ErrInvalidSignature struct {
	Vote *Vote
}

type ErrInvalidView struct {
	Vote *Vote
}

type ErrDoubleVote struct {
	OriginalVote *Vote
	DoubleVote   *Vote
}

type ErrStaleVote struct {
	Vote          *Vote
	FinalizedView uint64
}

type ErrStaleBlock struct {
	BlockProposal *BlockProposal
	FinalizedView uint64
}

type ErrExistingQC struct {
	Vote *Vote
	QC   *QuorumCertificate
}

type ErrInsufficientVotes struct {
}

func (e ErrorMissingSigner) Error() string {
	return fmt.Sprintf("The signer of vote %v is missing", e.Vote)
}

func (e ErrInvalidSignature) Error() string {
	return fmt.Sprintf("The signature of vote %v is invalid", e.Vote)
}

func (e ErrInvalidView) Error() string {
	return fmt.Sprintf("The view of vote %v is invalid", e.Vote)
}

func (e ErrDoubleVote) Error() string {
	return fmt.Sprintf("Double voting detected (original vote: %v, double vote: %v)", e.OriginalVote, e.DoubleVote)
}

func (e ErrStaleVote) Error() string {
	return fmt.Sprintf("Stale vote found (vote: %v, finalized view: %v)", e.Vote, e.FinalizedView)
}

func (e ErrStaleBlock) Error() string {
	return fmt.Sprintf("Stale block found (block: %v, finalized view: %v)", e.BlockProposal, e.FinalizedView)
}

func (e ErrExistingQC) Error() string {
	return fmt.Sprintf("QC already existed (vote: %v, qc: %v)", e.Vote, e.QC)
}

func (e ErrInsufficientVotes) Error() string {
	return fmt.Sprintf("Not receiving enough votes")
}

type ErrorConfiguration struct {
	Msg string
}

func (e *ErrorConfiguration) Error() string { return e.Msg }

type ErrorConflictingQCs struct {
	View uint64
	Qcs  []*QuorumCertificate
}

func (e *ErrorConflictingQCs) Error() string {
	return fmt.Sprintf("%d conflicting QCs at view %d", len(e.Qcs), e.View)
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

func (e *ErrorInvalidBlock) Error() string {
	return fmt.Sprintf("invalid block (view %d; ID %v): %s", e.View, e.BlockID, e.Msg)
}
