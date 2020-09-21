package protocol

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type Params interface {

	// ChainID returns the chain ID for the current consensus state.
	ChainID() (flow.ChainID, error)

	// Root returns the root header of the current consensus state.
	Root() (*flow.Header, error)

	// Seal returns the root block seal of the current consensus state.
	Seal() (*flow.Seal, error)
}
