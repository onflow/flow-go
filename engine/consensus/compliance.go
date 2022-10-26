package consensus

import (
	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
)

type Compliance interface {
	module.ReadyDoneAware
	module.Startable
	OnBlockProposal(proposal *messages.BlockProposal)
	OnSyncedBlock(block *events.SyncedBlock)
}
