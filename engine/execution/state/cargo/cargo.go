package cargo

import (
	"github.com/onflow/flow-go/engine/execution/state/cargo/payload"
	"github.com/onflow/flow-go/engine/execution/state/cargo/queue"
	"github.com/onflow/flow-go/engine/execution/state/cargo/storage"
	"github.com/onflow/flow-go/model/flow"
)

// Cargo is responsible for synchronization
// between a block execution and block finalization events
// block execution results and finalization are two concurrent
// streams and requires synchronization and buffering of events
// in case one of them is ahead of the other one.
// payloadStore accepts block execution events in a fork-aware way and expect block finalization
// signals to commit the changes into persistant storage.
// blockqueue acts as a fixed-size buffer to hold on to the unprocessed block finaliziation events
// both blockqueue and payloadstore has some internal validation to prevent
// out of order events and support concurency.
type Cargo struct {
	blockQueue   *queue.FinalizedBlockQueue
	payloadStore *payload.PayloadStore
}

func NewCargo(
	storage storage.Storage,
	blockQueueCapacity int,
	genesis *flow.Header,
) (*Cargo, error) {
	// TODO load genesis from storage
	payloadStore, err := payload.NewPayloadStore(storage)
	if err != nil {
		return nil, err
	}
	return &Cargo{
		blockQueue:   queue.NewFinalizedBlockQueue(blockQueueCapacity, genesis),
		payloadStore: payloadStore,
	}, nil
}

func (c *Cargo) Reader(header *flow.Header) *Reader {
	return NewReader(header, c.payloadStore.Get)
}

func (c *Cargo) BlockFinalized(new *flow.Header) error {
	// first enqueue the header
	// if we reach a capacity that we could not enqueue blocks and they stay uncommitable
	// then here we are returning an error
	// TODO(ramtin): switch case error types to take better action on higher level
	if err := c.blockQueue.Enqueue(new); err != nil {
		return err
	}

	// then trigger sync
	// we take a peak at the oldest block that has been finalized and not processed yet
	// and see if its commitable by payload storage (if results are available for that blockID),
	// if commitable (return true by payload store), we deuque the block and continue
	// doing the same for more blocks until payloadstore returns false, which mean it doesn't
	// have results for that block yet and we need to hold on until next trigger.
	blockID, header := c.blockQueue.Peak()
	for found, err := c.payloadStore.Commit(blockID, header); found; {
		if err != nil {
			return err
		}
		c.blockQueue.Dequeue()
		blockID, header = c.blockQueue.Peak()
	}

	return nil
}

func (c *Cargo) BlockExecuted(header *flow.Header, updates map[flow.RegisterID]flow.RegisterValue) error {
	// just add update to the payload store
	c.payloadStore.Update(header, updates)
	// we don't trigger actions here just collect in the next block finalized we deal with the gap
	return nil
}
