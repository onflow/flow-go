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

type ChunkExecutionDataOption func(*ChunkExecutionDataGenerator, *chunkExecutionDataConfig)

type chunkExecutionDataConfig struct {
	startTxIndex uint32
	ced          *execution_data.ChunkExecutionData
}

// WithCollection is an option that sets the collection for the chunk execution data.
func (f chunkExecutionDataFactory) WithCollection(collection *flow.Collection) ChunkExecutionDataOption {
	return func(g *ChunkExecutionDataGenerator, config *chunkExecutionDataConfig) {
		config.ced.Collection = collection
	}
}

// WithEvents is an option that sets the events for the chunk execution data.
func (f chunkExecutionDataFactory) WithEvents(events flow.EventsList) ChunkExecutionDataOption {
	return func(g *ChunkExecutionDataGenerator, config *chunkExecutionDataConfig) {
		config.ced.Events = events
	}
}

// WithTrieUpdate is an option that sets the trie update for the chunk execution data.
func (f chunkExecutionDataFactory) WithTrieUpdate(trieUpdate *ledger.TrieUpdate) ChunkExecutionDataOption {
	return func(g *ChunkExecutionDataGenerator, config *chunkExecutionDataConfig) {
		config.ced.TrieUpdate = trieUpdate
	}
}

// WithTransactionResults is an option that sets the transaction results for the chunk execution data.
func (f chunkExecutionDataFactory) WithTransactionResults(results ...flow.LightTransactionResult) ChunkExecutionDataOption {
	return func(g *ChunkExecutionDataGenerator, config *chunkExecutionDataConfig) {
		config.ced.TransactionResults = results
	}
}

// WithMinSize is an option that sets the minimum size for the chunk execution data.
func (f chunkExecutionDataFactory) WithMinSize(minSize int) ChunkExecutionDataOption {
	return func(g *ChunkExecutionDataGenerator, config *chunkExecutionDataConfig) {
		if minSize > 0 && config.ced.TrieUpdate != nil {
			g.ensureMinSize(config.ced, minSize)
		}
	}
}

// WithStartTxIndex is an option that sets the start transaction index for the chunk execution data.
// Use this to ensure the transaction index within events is consistent across chunks.
func (f chunkExecutionDataFactory) WithStartTxIndex(startTxIndex uint32) ChunkExecutionDataOption {
	return func(g *ChunkExecutionDataGenerator, config *chunkExecutionDataConfig) {
		config.startTxIndex = startTxIndex
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
	config := &chunkExecutionDataConfig{
		ced: &execution_data.ChunkExecutionData{
			Collection: g.collections.Fixture(Collection.WithTxCount(5)),
			TrieUpdate: g.trieUpdates.Fixture(),
		},
	}

	for _, opt := range opts {
		opt(g, config)
	}

	if len(config.ced.TransactionResults) == 0 {
		config.ced.TransactionResults = make([]flow.LightTransactionResult, len(config.ced.Collection.Transactions))
		for i, tx := range config.ced.Collection.Transactions {
			config.ced.TransactionResults[i] = g.lightTxResults.Fixture(LightTransactionResult.WithTransactionID(tx.ID()))
		}
	}

	if len(config.ced.Events) == 0 {
		for i, result := range config.ced.TransactionResults {
			txIndex := config.startTxIndex + uint32(i)
			events := g.events.ForTransaction(result.TransactionID, txIndex, 5)
			config.ced.Events = append(config.ced.Events, events...)
		}
		config.ced.Events = AdjustEventsMetadata(config.ced.Events)
	}

	return config.ced
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
