package indexer

import (
	"fmt"

	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// IndexerCore indexes the execution state.
type IndexerCore struct {
	registers storage.RegisterIndex
	headers   storage.Headers
	events    storage.Events
	log       zerolog.Logger
}

// New execution state indexer used to ingest block execution data and index it by height.
// The passed RegisterIndex storage must be populated to include the first and last height otherwise the indexer
// won't be initialized to ensure we have bootstrapped the storage first.
func New(
	log zerolog.Logger,
	registers storage.RegisterIndex,
	headers storage.Headers,
	events storage.Events,
) (*IndexerCore, error) {
	return &IndexerCore{
		registers: registers,
		headers:   headers,
		events:    events,
		log:       log.With().Str("component", "execution_indexer").Logger(),
	}, nil
}

// RegisterValues retrieves register values by the register IDs at the provided block height.
// Even if the register wasn't indexed at the provided height, returns the highest height the register was indexed at.
// Expected errors:
// - storage.ErrNotFound if the register by the ID was never indexed
func (c *IndexerCore) RegisterValues(IDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error) {
	values := make([]flow.RegisterValue, len(IDs))

	for j, id := range IDs {
		value, err := c.registers.Get(id, height)
		if err != nil {
			return nil, err
		}

		values[j] = value
	}

	return values, nil
}

// IndexBlockData indexes all execution block data by height.
// This method shouldn't be used concurrently.
// Expected errors:
// - storage.ErrNotFound if the block for execution data was not found
func (c *IndexerCore) IndexBlockData(data *execution_data.BlockExecutionDataEntity) error {
	block, err := c.headers.ByBlockID(data.BlockID)
	if err != nil {
		return fmt.Errorf("could not get the block by ID %s: %w", data.BlockID, err)
	}

	lg := c.log.With().
		Hex("block_id", logging.ID(data.BlockID)).
		Uint64("height", block.Height).
		Logger()

	lg.Debug().Msgf("indexing new block")

	// the height we are indexing must be exactly one bigger or same as the latest height indexed from the storage
	latest := c.registers.LatestHeight()
	if block.Height != latest+1 && block.Height != latest {
		return fmt.Errorf("must store registers with the next height %d, but got %d", latest+1, block.Height)
	}
	// allow rerunning the indexer for same height since we are fetching height from register storage, but there are other storages
	// for indexing resources which might fail to update the values, so this enables rerunning and reindexing those resources
	if block.Height == latest {
		lg.Warn().Msg("reindexing block data")
	}

	// concurrently process indexing of block data
	g := errgroup.Group{}

	g.Go(func() error {
		events := make([]flow.Event, 0)
		for _, chunk := range data.ChunkExecutionDatas {
			events = append(events, chunk.Events...)
		}

		err := c.indexEvents(data.BlockID, events)
		if err != nil {
			return fmt.Errorf("could not index events at height %d: %w", block.Height, err)
		}
		return nil
	})

	g.Go(func() error {
		// we are iterating all the registers and overwrite any existing register at the same path
		// this will make sure if we have multiple register changes only the last change will get persisted
		// if block has two chucks:
		// first chunk updates: { X: 1, Y: 2 }
		// second chunk updates: { X: 2 }
		// then we should persist only {X: 2: Y: 2}
		payloads := make(map[ledger.Path]*ledger.Payload)
		for _, chunk := range data.ChunkExecutionDatas {
			update := chunk.TrieUpdate
			if update != nil {
				// this should never happen but we check anyway
				if len(update.Paths) != len(update.Payloads) {
					return fmt.Errorf("update paths length is %d and payloads length is %d and they don't match", len(update.Paths), len(update.Payloads))
				}

				for i, path := range update.Paths {
					payloads[path] = update.Payloads[i]
				}
			}
		}

		err = c.indexRegisters(payloads, block.Height)
		if err != nil {
			return fmt.Errorf("could not index register payloads at height %d: %w", block.Height, err)
		}

		lg.Debug().
			Int("register_count", len(payloads)).
			Msg("indexed registers")

		return nil
	})

	err = g.Wait()
	if err != nil {
		return fmt.Errorf("failed to index block data at height %d: %w", block.Height, err)
	}
	return nil
}

func (c *IndexerCore) indexEvents(blockID flow.Identifier, events flow.EventsList) error {
	// Note: the last chunk in an execution data is the system chunk. All events in that ChunkExecutionData are service events.
	err := c.events.Store(blockID, []flow.EventsList{events})
	return err
}

func (c *IndexerCore) indexRegisters(registers map[ledger.Path]*ledger.Payload, height uint64) error {
	regEntries := make(flow.RegisterEntries, 0, len(registers))

	for _, payload := range registers {
		k, err := payload.Key()
		if err != nil {
			return err
		}

		id, err := convert.LedgerKeyToRegisterID(k)
		if err != nil {
			return err
		}

		regEntries = append(regEntries, flow.RegisterEntry{
			Key:   id,
			Value: payload.Value(),
		})
	}

	return c.registers.Store(regEntries, height)
}
