package types

import (
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

type Vote int

const (
	REJECT Vote = iota
	APPROVE
)

type StateTransition struct {
	PreviousStateTransitionHash      crypto.Hash
	PreviousCommitApprovalSignatures []crypto.Signature
	Height                           uint64
	Value                            []byte
}

type SignedStateTransition struct {
	StateTransition StateTransition
	Signature       crypto.Signature
}

type FinalizedStateTransition struct {
	SignedStateTransition SignedStateTransition
	Signatures            []crypto.Signature
}

type StateTransitionVote struct {
	StateTransitionHash crypto.Hash
	Vote                Vote
	Height              uint64
}

type SignedStateTransitionPrepareVote struct {
	StateTransitionVote StateTransitionVote
	Signature           crypto.Signature
}

type SignedStateTransitionCommitVote struct {
	StateTransitionVote StateTransitionVote
	Signature           crypto.Signature
}

func (v Vote) String() string {
	return [...]string{"REJECT", "APPROVE"}[v]
}
