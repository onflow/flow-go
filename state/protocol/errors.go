package protocol

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

var (
	// ErrNoPreviousEpoch is a sentinel error returned when a previous epoch is
	// queried from a snapshot within the first epoch after the root block.
	ErrNoPreviousEpoch = fmt.Errorf("no previous epoch exists")

	// ErrNextEpochNotSetup is a sentinal error returned when
	ErrNextEpochNotSetup = fmt.Errorf("next epoch has not yet been set up")
)

type IdentityNotFoundError struct {
	NodeID flow.Identifier
}

func (e IdentityNotFoundError) Error() string {
	return fmt.Sprintf("identity not found (%x)", e.NodeID)
}

func IsIdentityNotFound(err error) bool {
	var errIdentityNotFound IdentityNotFoundError
	return errors.As(err, &errIdentityNotFound)
}
