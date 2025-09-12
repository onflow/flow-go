package fixtures

import (
	"bytes"

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

func NewChunkExecutionDataGenerator(
	randomGen *RandomGenerator,
	collectionGen *CollectionGenerator,
	lightTransactionResultGen *LightTransactionResultGenerator,
	eventGen *EventGenerator,
	trieUpdateGen *TrieUpdateGenerator,
) *ChunkExecutionDataGenerator {
	return &ChunkExecutionDataGenerator{
		randomGen:                 randomGen,
		collectionGen:             collectionGen,
		lightTransactionResultGen: lightTransactionResultGen,
		eventGen:                  eventGen,
		trieUpdateGen:             trieUpdateGen,
	}
}

// WithCollection is an option that sets the collection for the chunk execution data.
func (g *ChunkExecutionDataGenerator) WithCollection(collection *flow.Collection) func(*execution_data.ChunkExecutionData) {
	return func(ced *execution_data.ChunkExecutionData) {
		ced.Collection = collection
	}
}

// WithEvents is an option that sets the events for the chunk execution data.
func (g *ChunkExecutionDataGenerator) WithEvents(events flow.EventsList) func(*execution_data.ChunkExecutionData) {
	return func(ced *execution_data.ChunkExecutionData) {
		ced.Events = events
	}
}

// WithTrieUpdate is an option that sets the trie update for the chunk execution data.
func (g *ChunkExecutionDataGenerator) WithTrieUpdate(trieUpdate *ledger.TrieUpdate) func(*execution_data.ChunkExecutionData) {
	return func(ced *execution_data.ChunkExecutionData) {
		ced.TrieUpdate = trieUpdate
	}
}

// WithTransactionResults is an option that sets the transaction results for the chunk execution data.
func (g *ChunkExecutionDataGenerator) WithTransactionResults(results []flow.LightTransactionResult) func(*execution_data.ChunkExecutionData) {
	return func(ced *execution_data.ChunkExecutionData) {
		ced.TransactionResults = results
	}
}

// WithMinSize is an option that sets the minimum size for the chunk execution data.
func (g *ChunkExecutionDataGenerator) WithMinSize(minSize int) func(*execution_data.ChunkExecutionData) {
	return func(ced *execution_data.ChunkExecutionData) {
		if minSize > 0 && ced.TrieUpdate != nil {
			g.ensureMinSize(ced, minSize)
		}
	}
}

// Fixture generates a [execution_data.ChunkExecutionData] with random data based on the provided options.
func (g *ChunkExecutionDataGenerator) Fixture(opts ...func(*execution_data.ChunkExecutionData)) *execution_data.ChunkExecutionData {
	ced := &execution_data.ChunkExecutionData{
		Collection: g.collectionGen.Fixture(g.collectionGen.WithTxCount(5)),
		TrieUpdate: g.trieUpdateGen.Fixture(),
	}

	for _, opt := range opts {
		opt(ced)
	}

	if len(ced.TransactionResults) == 0 {
		ced.TransactionResults = make([]flow.LightTransactionResult, len(ced.Collection.Transactions))
		for i, tx := range ced.Collection.Transactions {
			ced.TransactionResults[i] = g.lightTransactionResultGen.Fixture(g.lightTransactionResultGen.WithTransactionID(tx.ID()))
		}
	}

	if len(ced.Events) == 0 {
		for txIndex, result := range ced.TransactionResults {
			events := g.eventGen.List(5,
				g.eventGen.WithTransactionID(result.TransactionID),
				g.eventGen.WithTransactionIndex(uint32(txIndex)),
			)
			ced.Events = append(ced.Events, events...)
		}
		ced.Events = AdjustEventsMetadata(ced.Events)
	}

	return ced
}

// List generates a list of [execution_data.ChunkExecutionData].
func (g *ChunkExecutionDataGenerator) List(n int, opts ...func(*execution_data.ChunkExecutionData)) []*execution_data.ChunkExecutionData {
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

		v := g.randomGen.RandomBytes(size)

		ced.TrieUpdate.Payloads[0] = ledger.NewPayload(k, v)
		size *= 2
	}
}
