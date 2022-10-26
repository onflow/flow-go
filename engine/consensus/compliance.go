package consensus

import (
	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/component"
)

type Compliance interface {
	component.Component
	OnBlockProposal(proposal *messages.BlockProposal)
	OnSyncedBlock(block *events.SyncedBlock)
}
