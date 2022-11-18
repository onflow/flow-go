package events

import (
	"github.com/onflow/flow-go/model/flow"
)

type SyncedBlock struct {
	OriginID flow.Identifier
	Block    *flow.Block
}
