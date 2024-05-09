package events

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
)

type SyncedBlock struct {
	OriginID flow.Identifier
	Block    messages.UntrustedBlock
}

type SyncedClusterBlock struct {
	OriginID flow.Identifier
	Block    messages.UntrustedClusterBlock
}
