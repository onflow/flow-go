package voteaggregator

import (
	"github.com/dapperlabs/flow-go/engine/consensus/HotStuff/types"
)

type signerNotExistError struct {
	msg string
}

type invalidSigError struct {
	signerIdx uint32
	msg       string
}

type doubleVoteError struct {
	originalVote *types.Vote
	doubleVote   *types.Vote
	msg          string
}

type invalidViewError struct {
	vote *types.Vote
	msg  string
}

type qcExistedError struct {
	vote *types.Vote
	qc   *types.QuorumCertificate
	msg  string
}

func (e signerNotExistError) Error() string {
	return e.msg
}

func (e invalidSigError) Error() string {
	return e.msg
}

func (e doubleVoteError) Error() string {
	return e.msg
}

func (e invalidViewError) Error() string {
	return e.msg
}

func (e qcExistedError) Error() string {
	return e.msg
}
