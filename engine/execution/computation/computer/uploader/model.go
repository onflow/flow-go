package uploader

import (
	"fmt"
	"io"

	"github.com/fxamacker/cbor/v2"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/entity"
)

type BlockData struct {
	Block       *flow.Block
	Collections []*entity.CompleteCollection
	TxResults   []*flow.TransactionResult
	Events      []*flow.Event
	TrieUpdates []*ledger.TrieUpdate
}

func ComputationResultToBlockData(computationResult *execution.ComputationResult) *BlockData {

	txResults := make([]*flow.TransactionResult, len(computationResult.TransactionResults))
	for i := 0; i < len(computationResult.TransactionResults); i++ {
		txResults[i] = &computationResult.TransactionResults[i]
	}

	events := make([]*flow.Event, 0)
	for _, eventsList := range computationResult.Events {
		for i := 0; i < len(eventsList); i++ {
			events = append(events, &eventsList[i])
		}
	}

	return &BlockData{
		Block:       computationResult.ExecutableBlock.Block,
		Collections: computationResult.ExecutableBlock.Collections(),
		TxResults:   txResults,
		Events:      events,
		TrieUpdates: computationResult.TrieUpdates,
	}
}

func WriteComputationResultsTo(computationResult *execution.ComputationResult, writer io.Writer) error {
	blockData := ComputationResultToBlockData(computationResult)

	mode, err := cbor.CoreDetEncOptions().EncMode()
	if err != nil {
		return fmt.Errorf("cannot create deterministic cbor encoding mode: %w", err)
	}
	encoder := mode.NewEncoder(writer)

	return encoder.Encode(blockData)
}
