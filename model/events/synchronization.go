package events

import (
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
)

type SyncedBlock struct {
	OriginID flow.Identifier
	Block    *flow.Block
}

type SyncedClusterBlock struct {
	OriginID flow.Identifier
	Block    *cluster.Block
}
