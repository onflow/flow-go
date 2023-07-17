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
	Block                *flow.Block
	Collections          []*entity.CompleteCollection
	TxResults            []*flow.TransactionResult
	Events               []*flow.Event
	TrieUpdates          []*ledger.TrieUpdate
	FinalStateCommitment flow.StateCommitment
}

func ComputationResultToBlockData(computationResult *execution.ComputationResult) *BlockData {

	AllResults := computationResult.AllTransactionResults()
	txResults := make([]*flow.TransactionResult, len(AllResults))
	for i := 0; i < len(AllResults); i++ {
		txResults[i] = &AllResults[i]
	}

	eventsList := computationResult.AllEvents()
	events := make([]*flow.Event, len(eventsList))
	for i := 0; i < len(eventsList); i++ {
		events[i] = &eventsList[i]
	}

	trieUpdates := make(
		[]*ledger.TrieUpdate,
		0,
		len(computationResult.ChunkExecutionDatas))
	for _, chunk := range computationResult.ChunkExecutionDatas {
		trieUpdates = append(trieUpdates, chunk.TrieUpdate)
	}

	return &BlockData{
		Block:                computationResult.ExecutableBlock.Block,
		Collections:          computationResult.ExecutableBlock.Collections(),
		TxResults:            txResults,
		Events:               events,
		TrieUpdates:          trieUpdates,
		FinalStateCommitment: computationResult.CurrentEndState(),
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
