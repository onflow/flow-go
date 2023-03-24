package requester

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/state_synchronization"
)

// ExecutionDataDistributor subscribes to execution data received events from the requester and
// distributes them to subscribers
type ExecutionDataDistributor struct {
	// lastBlockID is the block ID of the most recent execution data received
	lastBlockID flow.Identifier

	consumers []state_synchronization.OnExecutionDataReceivedConsumer
	lock      sync.Mutex
}

func NewExecutionDataDistributor() *ExecutionDataDistributor {
	return &ExecutionDataDistributor{}
}

// LastBlockID returns the block ID of the most recent execution data received
// Execution data is guaranteed to be received in height order
func (p *ExecutionDataDistributor) LastBlockID() flow.Identifier {
	p.lock.Lock()
	defer p.lock.Unlock()

	return p.lastBlockID
}

// AddOnExecutionDataReceivedConsumer adds a consumer to be notified when new execution data is received
func (p *ExecutionDataDistributor) AddOnExecutionDataReceivedConsumer(consumer state_synchronization.OnExecutionDataReceivedConsumer) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.consumers = append(p.consumers, consumer)
}

// OnExecutionDataReceived is called when new execution data is received
func (p *ExecutionDataDistributor) OnExecutionDataReceived(executionData *execution_data.BlockExecutionData) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.lastBlockID = executionData.BlockID

	for _, consumer := range p.consumers {
		consumer(executionData)
	}
}
