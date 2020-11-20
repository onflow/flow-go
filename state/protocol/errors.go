package protocol

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// ErrNoPreviousEpoch is a sentinel error returned when a previous epoch is
// queried from a snapshot within the first epoch after the root block.
var ErrNoPreviousEpoch = fmt.Errorf("no previous epoch exists")

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
