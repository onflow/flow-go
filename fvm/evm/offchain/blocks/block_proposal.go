package blocks

import (
	"github.com/onflow/flow-go/fvm/evm/events"
	"github.com/onflow/flow-go/fvm/evm/types"
)

func ReconstructProposal(
	blockEvent *events.BlockEventPayload,
	results []*types.Result,
) *types.BlockProposal {
	receipts := make([]types.LightReceipt, 0, len(results))
	txHashes := make(types.TransactionHashes, 0, len(results))

	for _, result := range results {
		receipts = append(receipts, *result.LightReceipt())
		txHashes = append(txHashes, result.TxHash)
	}

	return &types.BlockProposal{
		Block: types.Block{
			ParentBlockHash:     blockEvent.ParentBlockHash,
			Height:              blockEvent.Height,
			Timestamp:           blockEvent.Timestamp,
			TotalSupply:         blockEvent.TotalSupply.Big(),
			ReceiptRoot:         blockEvent.ReceiptRoot,
			TransactionHashRoot: blockEvent.TransactionHashRoot,
			TotalGasUsed:        blockEvent.TotalGasUsed,
			PrevRandao:          blockEvent.PrevRandao,
		},
		Receipts: receipts,
		TxHashes: txHashes,
	}
}
