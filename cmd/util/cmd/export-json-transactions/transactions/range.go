package transactions

// this package provides functions to query transactions for a block height range.

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type BlockData struct {
	BlockID     flow.Identifier   `json:"block_id"`
	Height      uint64            `json:"height"`
	Collections []*CollectionData `json:"collections"`
}

type CollectionData struct {
	Index        int            `json:"index"`
	Transactions []*Transaction `json:"transactions"`
}

type Transaction struct {
	TxID   string `json:"tx_id"`
	Index  int    `json:"tx_index"`
	Script string `json:"script"`
	Payer  string `json:"payer_address"`
}

type Finder struct {
	State       protocol.State
	Payloads    storage.Payloads
	Collections storage.Collections
}

// startHeight and endHeight defines the range to iterate through and find transactions for each height,
// the run function is to process the queried transactions for each height, most common use case is to
// use run to export the given block data to json.
func (f *Finder) RunByHeightRange(startHeight uint64, endHeight uint64, run func(*BlockData) error) error {
	if startHeight > endHeight {
		return fmt.Errorf("startHeight %v must be smaller than endHeight %v", startHeight, endHeight)
	}

	if startHeight == 0 {
		return fmt.Errorf("start-height must not be 0")
	}

	for height := startHeight; height <= endHeight; height++ {
		header, err := f.State.AtHeight(height).Head()
		if err != nil {
			return fmt.Errorf("could not find header by height %v: %w", height, err)
		}
		blockID := header.ID()
		cols, err := FindBlockTransactions(f.Payloads, f.Collections, blockID)
		if err != nil {
			return fmt.Errorf("could not find transactions for block %v at height %v:, err", blockID, height)
		}

		err = run(&BlockData{
			BlockID:     blockID,
			Height:      height,
			Collections: cols,
		})

		if err != nil {
			return fmt.Errorf("fail to run at height %v: %w", height, err)
		}
	}

	return nil
}

func (f *Finder) GetByHeightRange(startHeight uint64, endHeight uint64) ([]*BlockData, error) {
	blocks := make([]*BlockData, 0, endHeight-startHeight+1)
	buildBlocks := func(b *BlockData) error {
		blocks = append(blocks, b)
		return nil
	}
	err := f.RunByHeightRange(startHeight, endHeight, buildBlocks)
	if err != nil {
		return nil, fmt.Errorf("fail to run height range [%v,%v]: %w", startHeight, endHeight, err)
	}
	return blocks, nil
}

func FindBlockTransactions(
	payloads storage.Payloads,
	collections storage.Collections,
	blockID flow.Identifier,
) ([]*CollectionData, error) {
	cols := make([]*CollectionData, 0)

	payload, err := payloads.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not find block payload: %w", err)
	}
	for colIndex, guarantee := range payload.Guarantees {
		col, err := collections.ByID(guarantee.CollectionID)
		if err != nil {
			return nil, fmt.Errorf("could not find collection %v: %w", guarantee.CollectionID, err)
		}
		txs := make([]*Transaction, 0, len(col.Transactions))
		for txIndex, tx := range col.Transactions {
			txs = append(txs, &Transaction{
				TxID:   tx.ID().String(),
				Index:  txIndex,
				Script: string(tx.Script),
				Payer:  tx.Payer.String(),
			})
		}
		cols = append(cols, &CollectionData{
			Index:        colIndex,
			Transactions: txs,
		})
	}
	return cols, nil
}
