package computer

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine/execution/computation/result"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/provider"
	"github.com/onflow/flow-go/module/mempool/entity"
)

// finalizer consumes attested collections and
// prepares everything needed to generate an execution receipt
type finalizer struct {
	metrics   module.ExecutionMetrics
	committer ViewCommitter

	signer                module.Local
	spockHasher           hash.Hasher
	receiptHasher         hash.Hasher
	executionDataProvider *provider.Provider

	block                        *entity.ExecutableBlock
	chunks                       flow.ChunkList
	parentBlockExecutionResultID flow.Identifier
	consumers                    []result.ExecutedBlockConsumer
	spockSignatures              []crypto.Signature
	convertedServiceEvents       flow.ServiceEventList
}

func NewFinalizer(
	block *entity.ExecutableBlock,
	parentBlockExecutionResultID flow.Identifier,
	consumers []result.ExecutedBlockConsumer,
) *finalizer {
	numberOfChunks := len(block.CompleteCollections) + 1
	return &finalizer{
		block:                        block,
		chunks:                       make(flow.ChunkList, numberOfChunks),
		consumers:                    consumers,
		parentBlockExecutionResultID: parentBlockExecutionResultID,
	}
}

func (fin *finalizer) OnAttestedCollection(ac result.AttestedCollection) error {
	spock, err := fin.signer.SignFunc(
		ac.SpockData(),
		fin.spockHasher,
		SPOCKProve)
	if err != nil {
		return fmt.Errorf("signing spock hash failed: %w", err)
	}
	fin.spockSignatures = append(fin.spockSignatures, spock)

	fin.convertedServiceEvents = append(
		fin.convertedServiceEvents,
		ac.ServiceEventList()...)

	if ac.IsSystemCollection() {
		return fin.finalize()
	}
	return nil
}

func (fin *finalizer) finalize() error {

	// TODO (IMPORTANT) convertedServiceEvents

	executionDataID, err := fin.executionDataProvider.Provide(
		context.Background(),
		fin.block.Height(),
		fin.block.BlockExecutionData)
	if err != nil {
		return fmt.Errorf("failed to provide execution data: %w", err)
	}

	executionResult := flow.NewExecutionResult(
		fin.parentBlockExecutionResultID,
		fin.block.ID(),
		fin.chunks,
		fin.convertedServiceEvents,
		executionDataID)

	executionReceipt, err := GenerateExecutionReceipt(
		fin.signer,
		fin.receiptHasher,
		executionResult,
		fin.spockSignatures)
	if err != nil {
		return fmt.Errorf("could not sign execution result: %w", err)
	}

	fin.result.ExecutionReceipt = executionReceipt

	return fin.result, nil
}

func GenerateExecutionReceipt(
	signer module.Local,
	receiptHasher hash.Hasher,
	result *flow.ExecutionResult,
	spockSignatures []crypto.Signature,
) (
	*flow.ExecutionReceipt,
	error,
) {
	receipt := &flow.ExecutionReceipt{
		ExecutionResult:   *result,
		Spocks:            spockSignatures,
		ExecutorSignature: crypto.Signature{},
		ExecutorID:        signer.NodeID(),
	}

	// generates a signature over the execution result
	id := receipt.ID()
	sig, err := signer.Sign(id[:], receiptHasher)
	if err != nil {
		return nil, fmt.Errorf("could not sign execution result: %w", err)
	}

	receipt.ExecutorSignature = sig

	return receipt, nil
}
