package epochs

import (
	"errors"
	"fmt"
)

// ClusterQCNoVoteError is returned when a ClusterRootQCVoter fails to submit a vote
// for reasons we consider benign, either:
//   - we have reached a retry limit for transient failures
//   - we have decided to not submit the vote, for example because we are not in the EpochSetup phase
type ClusterQCNoVoteError struct {
	Err error
}

func (err ClusterQCNoVoteError) Error() string {
	return err.Err.Error()
}

func (err ClusterQCNoVoteError) Unwrap() error {
	return err.Err
}

func NewClusterQCNoVoteErrorf(msg string, args ...interface{}) ClusterQCNoVoteError {
	return ClusterQCNoVoteError{
		Err: fmt.Errorf(msg, args...),
	}
}

func IsClusterQCNoVoteError(err error) bool {
	var errClusterVotingFailed ClusterQCNoVoteError
	return errors.As(err, &errClusterVotingFailed)
}

// QCVoteStoppedError is returned when in progress voting is stopped by an outside process.
type QCVoteStoppedError struct {
	Err error
}

func (err QCVoteStoppedError) Error() string {
	return err.Err.Error()
}

func (err QCVoteStoppedError) Unwrap() error {
	return err.Err
}

func NewQCVoteStoppedError() QCVoteStoppedError {
	return QCVoteStoppedError{
		Err: fmt.Errorf("root qc voting exiting stop voting signal encountered"),
	}
}

func IsQCVoteStoppedError(err error) bool {
	var qcVoteStoppedErr QCVoteStoppedError
	return errors.As(err, &qcVoteStoppedErr)
}
