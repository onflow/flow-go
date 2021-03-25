package committees

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/state/protocol"
)

// convertError converts protocol errors to HotStuff errors.
func convertError(err error) error {
	if protocol.IsIdentityNotFound(err) {
		return model.ErrInvalidSigner
	}
	return err
}
