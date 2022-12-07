package events

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
)

// TODO: will be removed in favor of flow.Slashable
type GenericSyncedBlock[UntrustedType any] struct {
	OriginID flow.Identifier
	Block    UntrustedType
}

type SyncedBlock = GenericSyncedBlock[messages.UntrustedBlock]
type SyncedClusterBlock = GenericSyncedBlock[messages.UntrustedClusterBlock]
