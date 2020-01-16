package voteaggregator

import (
	"fmt"
	"github.com/dapperlabs/flow-go/engine/consensus/HotStuff/types"
)

type errorMissingSigner struct {
	vote *types.Vote
}

type errInvalidSignature struct {
	vote *types.Vote
}

type errInvalidView struct {
	vote *types.Vote
}

type errDoubleVote struct {
	originalVote *types.Vote
	doubleVote   *types.Vote
}

type errExistingQC struct {
	vote *types.Vote
	qc   *types.QuorumCertificate
}

func (e errorMissingSigner) Error() string {
	return fmt.Sprintf("The signer of vote %v is missing", e.vote)
}

func (e errInvalidSignature) Error() string {
	return fmt.Sprintf("The signature of vote %v is invalid", e.vote)
}

func (e errInvalidView) Error() string {
	return fmt.Sprintf("The view of vote %v is invalid", e.vote)
}

func (e errDoubleVote) Error() string {
	return fmt.Sprintf("Double voting detected (original vote: %v, double vote: %v)", e.originalVote, e.doubleVote)
}

func (e errExistingQC) Error() string {
	return fmt.Sprintf("QC already existed (vote: %v, qc: %v)", e.vote, e.qc)
}
