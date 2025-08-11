package fixtures

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

// ChunkExecutionDataGenerator generates chunk execution data with consistent randomness.
type ChunkExecutionDataGenerator struct {
	randomGen                 *RandomGenerator
	collectionGen             *CollectionGenerator
	lightTransactionResultGen *LightTransactionResultGenerator
	eventGen                  *EventGenerator
	trieUpdateGen             *TrieUpdateGenerator
}

// chunkExecutionDataConfig holds the configuration for chunk execution data generation.
type chunkExecutionDataConfig struct {
	collection         *flow.Collection
	events             flow.EventsList
	trieUpdate         *ledger.TrieUpdate
	transactionResults []flow.LightTransactionResult
	minSize            int
}

// WithCollection returns an option to set the collection for the chunk execution data.
func (g *ChunkExecutionDataGenerator) WithCollection(collection *flow.Collection) func(*chunkExecutionDataConfig) {
	return func(config *chunkExecutionDataConfig) {
		config.collection = collection
	}
}

// WithEvents returns an option to set the events for the chunk execution data.
func (g *ChunkExecutionDataGenerator) WithEvents(events flow.EventsList) func(*chunkExecutionDataConfig) {
	return func(config *chunkExecutionDataConfig) {
		config.events = events
	}
}

// WithTrieUpdate returns an option to set the trie update for the chunk execution data.
func (g *ChunkExecutionDataGenerator) WithTrieUpdate(trieUpdate *ledger.TrieUpdate) func(*chunkExecutionDataConfig) {
	return func(config *chunkExecutionDataConfig) {
		config.trieUpdate = trieUpdate
	}
}

// WithTransactionResults returns an option to set the transaction results for the chunk execution data.
func (g *ChunkExecutionDataGenerator) WithTransactionResults(results []flow.LightTransactionResult) func(*chunkExecutionDataConfig) {
	return func(config *chunkExecutionDataConfig) {
		config.transactionResults = results
	}
}

// WithMinSize returns an option to set the minimum size for the chunk execution data.
func (g *ChunkExecutionDataGenerator) WithMinSize(minSize int) func(*chunkExecutionDataConfig) {
	return func(config *chunkExecutionDataConfig) {
		config.minSize = minSize
	}
}

// Fixture generates chunk execution data with optional configuration.
func (g *ChunkExecutionDataGenerator) Fixture(t testing.TB, opts ...func(*chunkExecutionDataConfig)) *execution_data.ChunkExecutionData {
	config := &chunkExecutionDataConfig{
		collection: g.collectionGen.Fixture(t, 5),
		trieUpdate: g.trieUpdateGen.Fixture(t),
	}

	// Apply options
	for _, opt := range opts {
		opt(config)
	}

	if len(config.transactionResults) == 0 {
		config.transactionResults = make([]flow.LightTransactionResult, len(config.collection.Transactions))
		for i, tx := range config.collection.Transactions {
			config.transactionResults[i] = g.lightTransactionResultGen.Fixture(t, g.lightTransactionResultGen.WithTransactionID(tx.ID()))
		}
	}

	if len(config.events) == 0 {
		for txIndex, result := range config.transactionResults {
			events := g.eventGen.List(t, 5,
				g.eventGen.WithTransactionID(result.TransactionID),
				g.eventGen.WithTransactionIndex(uint32(txIndex)),
			)
			config.events = append(config.events, events...)
		}
		config.events = AdjustEventsMetadata(config.events)
	}

	ced := &execution_data.ChunkExecutionData{
		Collection:         config.collection,
		Events:             config.events,
		TrieUpdate:         config.trieUpdate,
		TransactionResults: config.transactionResults,
	}

	// Handle minimum size requirement
	if config.minSize > 0 && ced.TrieUpdate != nil {
		g.ensureMinSize(t, ced, config.minSize)
	}

	return ced
}

// List generates a list of chunk execution data.
func (g *ChunkExecutionDataGenerator) List(t testing.TB, n int, opts ...func(*chunkExecutionDataConfig)) []*execution_data.ChunkExecutionData {
	list := make([]*execution_data.ChunkExecutionData, n)
	for i := range n {
		list[i] = g.Fixture(t, opts...)
	}
	return list
}

// Helper methods for generating random values

func (g *ChunkExecutionDataGenerator) ensureMinSize(t testing.TB, ced *execution_data.ChunkExecutionData, minSize int) {
	size := 1
	for {
		buf := &bytes.Buffer{}
		err := execution_data.DefaultSerializer.Serialize(buf, ced)
		require.NoError(t, err)
		if buf.Len() >= minSize {
			return
		}

		v := g.randomGen.RandomBytes(t, size)

		k, err := ced.TrieUpdate.Payloads[0].Key()
		require.NoError(t, err)

		ced.TrieUpdate.Payloads[0] = ledger.NewPayload(k, v)
		size *= 2
	}
}
