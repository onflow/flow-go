package protocol

import (
	"errors"
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

type IdentityNotFoundErr struct {
	NodeID flow.Identifier
}

func (e IdentityNotFoundErr) Error() string {
	return fmt.Sprintf("identity not found (%x)", e.NodeID)
}

func IsIdentityNotFound(err error) bool {
	var errIdentityNotFound IdentityNotFoundErr
	return errors.As(err, &errIdentityNotFound)
}
