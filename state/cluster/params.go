package cluster

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type Params interface {

	// ChainID returns the chain ID for the current cluster.
	ChainID() (flow.ChainID, error)

	// Cluster returns the initial cluster participants.
	Cluster() (flow.IdentityList, error)

	// Epoch returns the number of the epoch this cluster is for.
	Epoch() (uint64, error)
}
