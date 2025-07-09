package requester

import (
	"sync"

	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/state_synchronization"
)

// ExecutionDataDistributor subscribes to execution data received events from the requester and
// distributes them to subscribers
type ExecutionDataDistributor struct {
	consumers []state_synchronization.OnExecutionDataReceivedConsumer
	lock      sync.RWMutex
}

func NewExecutionDataDistributor() *ExecutionDataDistributor {
	return &ExecutionDataDistributor{}
}

// AddOnExecutionDataReceivedConsumer adds a consumer to be notified when new execution data is received
func (p *ExecutionDataDistributor) AddOnExecutionDataReceivedConsumer(consumer state_synchronization.OnExecutionDataReceivedConsumer) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.consumers = append(p.consumers, consumer)
}

// OnExecutionDataReceived is called when new execution data is received
func (p *ExecutionDataDistributor) OnExecutionDataReceived(executionData *execution_data.BlockExecutionDataEntity) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	for _, consumer := range p.consumers {
		consumer(executionData)
	}
}
