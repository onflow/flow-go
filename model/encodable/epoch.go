package encodable

import (
	"github.com/onflow/flow-go/model/flow"
)

// Epochs is the encoding format for protocol.EpochQuery
type Epochs struct {
	Previous Epoch
	Current  Epoch
	Next     Epoch
}

// Epoch is the encoding format for protocol.Epoch
type Epoch struct {
	Counter           uint64
	FirstView         uint64
	FinalView         uint64
	RandomSource      []byte
	InitialIdentities flow.IdentityList
	Clusters          []Cluster
	DKG               DKG
}
