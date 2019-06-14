package security

import (
	"context"
	"time"

	"github.com/dapperlabs/bamboo-emulator/crypto"
	"github.com/dapperlabs/bamboo-emulator/data"
)

// BlockBuilder produces blocks from incoming collections and seals incoming blocks.
type BlockBuilder struct {
	state              *data.WorldState
	collectionsIn      <-chan *data.Collection
	pendingBlocksIn    <-chan *data.Block
	pendingCollections []*data.Collection
	pendingBlocks      []crypto.Hash
}

// NewBlockBuilder initializes a new BlockBuilder with the incoming collectionsIn channel.
//
// The BlockBuilder pulls collections from the collectionsIn channel and writes new blocks to the shared world state.
// The BlockBuilder also pulls blocks from the pendingBlocksIn channel and seals them.
func NewBlockBuilder(state *data.WorldState, collectionsIn <-chan *data.Collection, pendingBlocksIn <-chan *data.Block) *BlockBuilder {
	return &BlockBuilder{
		state:              state,
		collectionsIn:      collectionsIn,
		pendingBlocksIn:    pendingBlocksIn,
		pendingCollections: []*data.Collection{},
		pendingBlocks:      []crypto.Hash{},
	}
}

// Start starts the block builder worker loop.
func (b *BlockBuilder) Start(ctx context.Context) {
	tick := time.Tick(time.Second)
	for {
		select {
		case <-tick:
			b.buildBlock()
			b.sealBlocks()
		case col := <-b.collectionsIn:
			b.enqueueCollection(col)
		case block := <-b.pendingBlocksIn:
			b.enqueueBlock(block)
		case <-ctx.Done():
			return
		}
	}
}

func (b *BlockBuilder) enqueueCollection(col *data.Collection) {
	b.pendingCollections = append(b.pendingCollections, col)
}

func (b *BlockBuilder) enqueueBlock(block *data.Block) {
	b.pendingBlocks = append(b.pendingBlocks, block.Hash())
}

func (b *BlockBuilder) sealBlocks() {
	if len(b.pendingBlocks) > 0 {
		for _, blockHash := range b.pendingBlocks {
			b.state.SealBlock(blockHash)
		}

		b.pendingBlocks = []crypto.Hash{}
	}
}

func (b *BlockBuilder) buildBlock() {
	if len(b.pendingCollections) == 0 {
		b.mintNoOpBlock()
	} else {
		b.mintNewBlock()
		b.pendingCollections = []*data.Collection{}
	}
}

func (b *BlockBuilder) mintNoOpBlock() error {
	latestBlock, _ := b.state.GetLatestBlock()

	newBlock := &data.Block{
		Number:            latestBlock.Number,
		Timestamp:         time.Now(),
		PrevBlockHash:     latestBlock.Hash(),
		Status:            data.BlockPending,
		CollectionHashes:  []crypto.Hash{},
		TransactionHashes: []crypto.Hash{},
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

	return nil
}

func (b *BlockBuilder) mintNewBlock() error {
	latestBlock, _ := b.state.GetLatestBlock()

	collectionHashes := make([]crypto.Hash, len(b.pendingCollections))
	transactionHashes := []crypto.Hash{}

	for i, col := range b.pendingCollections {
		collectionHashes[i] = col.Hash()
		collectionTxHashes := col.TransactionHashes
		transactionHashes = append(transactionHashes, collectionTxHashes...)
	}

	newBlock := &data.Block{
		Number:            latestBlock.Number,
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

	return nil
}
