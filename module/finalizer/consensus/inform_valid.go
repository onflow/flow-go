package consensus

import (
	"github.com/onflow/flow-go/model/flow"
)

// InformValidFunc is called by the MakeValid callback to allow components to
// react when new blocks are validated.
type InformValidFunc func(blockID flow.Identifier) error

// InformNothing is an InformFunc that does nothing.
func InformNothing() InformValidFunc {
	return func(flow.Identifier) error {
		return nil
	}
}
