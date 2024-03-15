package cluster

import (
	"github.com/onflow/flow-go/model/flow"
)

// Params contains constant information about this cluster state.
type Params interface {
	// ChainID returns the chain ID for this cluster.
	ChainID() flow.ChainID
}
