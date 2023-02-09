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
