package requester

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/state_synchronization"
)

// ExecutionDataDistributor subscribes execution data received events from the requester and
// distributes it to subscribers
type ExecutionDataDistributor struct {
	// lastBlockID is the block ID of the most recent execution data received
	lastBlockID flow.Identifier

	consumers []state_synchronization.OnExecutionDataReceivedConsumer
	lock      sync.Mutex
}

func NewExecutionDataDistributor() *ExecutionDataDistributor {
	return &ExecutionDataDistributor{}
}

func (p *ExecutionDataDistributor) LastBlockID() flow.Identifier {
	p.lock.Lock()
	defer p.lock.Unlock()

	return p.lastBlockID
}

func (p *ExecutionDataDistributor) AddOnExecutionDataReceivedConsumer(consumer state_synchronization.OnExecutionDataReceivedConsumer) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.consumers = append(p.consumers, consumer)
}

func (p *ExecutionDataDistributor) OnExecutionDataReceived(executionData *execution_data.BlockExecutionData) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.lastBlockID = executionData.BlockID

	for _, consumer := range p.consumers {
		consumer(executionData)
	}
}
