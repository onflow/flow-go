package stdmap

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/mempool/queue"
	_ "github.com/onflow/flow-go/utils/binstat"
)

type Queues struct {
	*Backend
}

// QueuesBackdata is mempool map for ingestion.Queues (head Node ID -> Queues)
type QueuesBackdata struct {
	mempool.BackData
}

func NewQueues() *Queues {
	return &Queues{NewBackend(WithEject(EjectPanic))}
}

func (b *QueuesBackdata) ByID(queueID flow.Identifier) (*queue.Queue, bool) {
	entity, exists := b.BackData.ByID(queueID)
	if !exists {
		return nil, false
	}
	queue := entity.(*queue.Queue)
	return queue, true
}

func (b *QueuesBackdata) All() []*queue.Queue {
	entities := b.BackData.All()

	queues := make([]*queue.Queue, len(entities))
	i := 0
	for _, entity := range entities {
		queue, ok := entity.(*queue.Queue)
		if !ok {
			panic(fmt.Sprintf("invalid entity in queue mempool (%T)", entity))
		}
		queues[i] = queue
		i++
	}
	return queues
}

func (b *Queues) Add(queue *queue.Queue) bool {
	return b.Backend.Add(queue)
}

func (b *Queues) Get(queueID flow.Identifier) (*queue.Queue, bool) {
	backdata := &QueuesBackdata{b.backData}
	return backdata.ByID(queueID)
}

func (b *Queues) Run(f func(backdata *QueuesBackdata) error) error {
	//bs1 := binstat.EnterTime(binstat.BinStdmap + ".w_lock.(Queues)Run")
	b.Lock()
	//binstat.Leave(bs1)

	//bs2 := binstat.EnterTime(binstat.BinStdmap + ".inlock.(Queues)Run")
	defer b.Unlock()
	err := f(&QueuesBackdata{b.backData})
	//binstat.Leave(bs2)

	if err != nil {
		return err
	}
	return nil
}
