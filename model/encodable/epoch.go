package encodable

import (
	"github.com/onflow/flow-go/model/flow"
)

type Epochs struct {
	Previous Epoch
	Current  Epoch
	Next     Epoch
}

type Epoch struct {
	Counter           uint64
	FirstView         uint64
	FinalView         uint64
	Seed              []byte
	InitialIdentities flow.IdentityList
	Clusters          []Cluster
	DKG               DKG
}
