package block_builder

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dapperlabs/bamboo-emulator/crypto"
	"github.com/dapperlabs/bamboo-emulator/data"
)

// BlockBuilder produces blocks from incoming collections.
type BlockBuilder struct {
	state              *data.WorldState
	collectionsIn      <-chan *data.Collection
	pendingCollections []*data.Collection
	log                 *logrus.Logger
}

// NewBlockBuilder initializes a new BlockBuilder with the incoming collectionsIn channel.
//
// The BlockBuilder pulls collections from the collectionsIn channel and writes new blocks to the shared world state.
func NewBlockBuilder(state *data.WorldState, collectionsIn <-chan *data.Collection, log *logrus.Logger) *BlockBuilder {
	return &BlockBuilder{
		state:              state,
		collectionsIn:      collectionsIn,
		pendingCollections: []*data.Collection{},
		log:                log,
	}
}

// Start starts the block builder worker loop.
func (b *BlockBuilder) Start(ctx context.Context, interval time.Duration) {
	tick := time.Tick(interval)
	for {
		select {
		case <-tick:
			b.mintNewBlock()
		case col := <-b.collectionsIn:
			b.enqueueCollection(col)
		case <-ctx.Done():
			return
		}
	}
}

func (b *BlockBuilder) enqueueCollection(col *data.Collection) {
	b.pendingCollections = append(b.pendingCollections, col)
}

func (b *BlockBuilder) bundleCollections() ([]crypto.Hash, []crypto.Hash) {
	if len(b.pendingCollections) == 0 {
		return []crypto.Hash{}, []crypto.Hash{}
	}

	collectionHashes := make([]crypto.Hash, len(b.pendingCollections))
	transactionHashes := []crypto.Hash{}

	for i, col := range b.pendingCollections {
		collectionHashes[i] = col.Hash()
		collectionTxHashes := col.TransactionHashes
		transactionHashes = append(transactionHashes, collectionTxHashes...)
	}

	b.pendingCollections = []*data.Collection{}

	return collectionHashes, transactionHashes
}

func (b *BlockBuilder) mintNewBlock() error {
	latestBlock := b.state.GetLatestBlock()

	collectionHashes, transactionHashes := b.bundleCollections()

	newBlock := &data.Block{
		Number:            latestBlock.Number + 1,
		Timestamp:         time.Now(),
		PrevBlockHash:     latestBlock.Hash(),
		Status:            data.BlockPending,
		CollectionHashes:  collectionHashes,
		TransactionHashes: transactionHashes,
	}

	err := b.state.AddBlock(newBlock)

	if err != nil {
		switch err.(type) {
		case *data.DuplicateItemError:
			return &DuplicateBlockError{blockHash: newBlock.Hash()}
		default:
			return err
		}
	}

	b.log.
		WithFields(logrus.Fields{
			"blockNum": newBlock.Number,
			"blockHash": newBlock.Hash(),
			"numCollections": len(collectionHashes),
			"numTransactions": len(transactionHashes),
		}).
		Infof(
			"Publishing block %d (0x%v) with %d transaction(s)",
			newBlock.Number,
			newBlock.Hash(),
			len(transactionHashes),
		)

	return nil
}
