package types

import "fmt"

type ErrorFinalizationFatal struct {
}

func (e *ErrorFinalizationFatal) Error() string {
	panic("implement me")
}

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

func (e ErrExistingQC) Error() string {
	return fmt.Sprintf("QC already existed (vote: %v, qc: %v)", e.Vote, e.QC)
}

func (e ErrInsufficientVotes) Error() string {
	return fmt.Sprintf("Not receiving enough votes")
}
