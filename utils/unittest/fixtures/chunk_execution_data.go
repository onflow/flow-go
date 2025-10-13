package fixtures

import (
	"bytes"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

// ChunkExecutionData is the default options factory for [execution_data.ChunkExecutionData] generation.
var ChunkExecutionData chunkExecutionDataFactory

type chunkExecutionDataFactory struct{}

type ChunkExecutionDataOption func(*ChunkExecutionDataGenerator, *execution_data.ChunkExecutionData)

// WithCollection is an option that sets the collection for the chunk execution data.
func (f chunkExecutionDataFactory) WithCollection(collection *flow.Collection) ChunkExecutionDataOption {
	return func(g *ChunkExecutionDataGenerator, ced *execution_data.ChunkExecutionData) {
		ced.Collection = collection
	}
}

// WithEvents is an option that sets the events for the chunk execution data.
func (f chunkExecutionDataFactory) WithEvents(events flow.EventsList) ChunkExecutionDataOption {
	return func(g *ChunkExecutionDataGenerator, ced *execution_data.ChunkExecutionData) {
		ced.Events = events
	}
}

// WithTrieUpdate is an option that sets the trie update for the chunk execution data.
func (f chunkExecutionDataFactory) WithTrieUpdate(trieUpdate *ledger.TrieUpdate) ChunkExecutionDataOption {
	return func(g *ChunkExecutionDataGenerator, ced *execution_data.ChunkExecutionData) {
		ced.TrieUpdate = trieUpdate
	}
}

// WithTransactionResults is an option that sets the transaction results for the chunk execution data.
func (f chunkExecutionDataFactory) WithTransactionResults(results ...flow.LightTransactionResult) ChunkExecutionDataOption {
	return func(g *ChunkExecutionDataGenerator, ced *execution_data.ChunkExecutionData) {
		ced.TransactionResults = results
	}
}

// WithMinSize is an option that sets the minimum size for the chunk execution data.
func (f chunkExecutionDataFactory) WithMinSize(minSize int) ChunkExecutionDataOption {
	return func(g *ChunkExecutionDataGenerator, ced *execution_data.ChunkExecutionData) {
		if minSize > 0 && ced.TrieUpdate != nil {
			g.ensureMinSize(ced, minSize)
		}
	}
}

// ChunkExecutionDataGenerator generates chunk execution data with consistent randomness.
type ChunkExecutionDataGenerator struct {
	chunkExecutionDataFactory

	random         *RandomGenerator
	collections    *CollectionGenerator
	lightTxResults *LightTransactionResultGenerator
	events         *EventGenerator
	trieUpdates    *TrieUpdateGenerator
}

func NewChunkExecutionDataGenerator(
	random *RandomGenerator,
	collections *CollectionGenerator,
	lightTxResults *LightTransactionResultGenerator,
	events *EventGenerator,
	trieUpdates *TrieUpdateGenerator,
) *ChunkExecutionDataGenerator {
	return &ChunkExecutionDataGenerator{
		random:         random,
		collections:    collections,
		lightTxResults: lightTxResults,
		events:         events,
		trieUpdates:    trieUpdates,
	}
}

// Fixture generates a [execution_data.ChunkExecutionData] with random data based on the provided options.
func (g *ChunkExecutionDataGenerator) Fixture(opts ...ChunkExecutionDataOption) *execution_data.ChunkExecutionData {
	ced := &execution_data.ChunkExecutionData{
		Collection: g.collections.Fixture(Collection.WithTxCount(5)),
		TrieUpdate: g.trieUpdates.Fixture(),
	}

	for _, opt := range opts {
		opt(g, ced)
	}

	if len(ced.TransactionResults) == 0 {
		ced.TransactionResults = make([]flow.LightTransactionResult, len(ced.Collection.Transactions))
		for i, tx := range ced.Collection.Transactions {
			ced.TransactionResults[i] = g.lightTxResults.Fixture(LightTransactionResult.WithTransactionID(tx.ID()))
		}
	}

	if len(ced.Events) == 0 {
		for txIndex, result := range ced.TransactionResults {
			events := g.events.List(5,
				Event.WithTransactionID(result.TransactionID),
				Event.WithTransactionIndex(uint32(txIndex)),
			)
			ced.Events = append(ced.Events, events...)
		}
		ced.Events = AdjustEventsMetadata(ced.Events)
	}

	return ced
}

// List generates a list of [execution_data.ChunkExecutionData].
func (g *ChunkExecutionDataGenerator) List(n int, opts ...ChunkExecutionDataOption) []*execution_data.ChunkExecutionData {
	list := make([]*execution_data.ChunkExecutionData, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}

// Helper methods for generating random values

func (g *ChunkExecutionDataGenerator) ensureMinSize(ced *execution_data.ChunkExecutionData, minSize int) {
	size := 1
	for {
		buf := &bytes.Buffer{}
		err := execution_data.DefaultSerializer.Serialize(buf, ced)
		NoError(err)

		if buf.Len() >= minSize {
			return
		}

		k, err := ced.TrieUpdate.Payloads[0].Key()
		NoError(err)

		v := g.random.RandomBytes(size)

		ced.TrieUpdate.Payloads[0] = ledger.NewPayload(k, v)
		size *= 2
	}
}
