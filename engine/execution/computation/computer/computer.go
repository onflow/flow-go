package computer

import (
	"context"
	"fmt"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"

	"github.com/dapperlabs/flow-go/engine/execution"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool/entity"
	"github.com/dapperlabs/flow-go/module/trace"
	"github.com/dapperlabs/flow-go/storage"
)

// A BlockComputer executes the transactions in a block.
type BlockComputer interface {
	ExecuteBlock(context.Context, *entity.ExecutableBlock, *delta.View) (*execution.ComputationResult, error)
}

type blockComputer struct {
	metrics module.ExecutionMetrics
	tracer  module.Tracer
	vm      virtualmachine.VirtualMachine
	blocks  storage.Blocks
}

// NewBlockComputer creates a new block executor.
func NewBlockComputer(
	metrics module.ExecutionMetrics,
	tracer module.Tracer,
	vm virtualmachine.VirtualMachine,
	blocks storage.Blocks,
) BlockComputer {
	return &blockComputer{
		metrics: metrics,
		tracer:  tracer,
		vm:      vm,
		blocks:  blocks,
	}
}

// ExecuteBlock executes a block and returns the resulting chunks.
func (e *blockComputer) ExecuteBlock(
	ctx context.Context,
	block *entity.ExecutableBlock,
	stateView *delta.View,
) (*execution.ComputationResult, error) {

	if e.tracer != nil {
		span, _ := e.tracer.StartSpanFromContext(ctx, trace.EXEComputeBlock)
		defer span.Finish()
	}

	results, err := e.executeBlock(ctx, block, stateView)
	if err != nil {
		return nil, fmt.Errorf("failed to execute transactions: %w", err)
	}

	// TODO: compute block fees & reward payments

	return results, nil
}

func (e *blockComputer) executeBlock(
	ctx context.Context,
	block *entity.ExecutableBlock,
	stateView *delta.View,
) (*execution.ComputationResult, error) {

	blockCtx := e.vm.NewBlockContext(block.Block.Header, e.blocks)

	collections := block.Collections()

	var gasUsed uint64

	interactions := make([]*delta.Snapshot, len(collections))

	events := make([]flow.Event, 0)
	blockTxResults := make([]flow.TransactionResult, 0)

	var txIndex uint32

	for i, collection := range collections {

		collectionView := stateView.NewChild()

		collEvents, txResults, nextIndex, gas, err := e.executeCollection(
			ctx, txIndex, blockCtx, collectionView, collection,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to execute collection: %w", err)
		}

		gasUsed += gas

		txIndex = nextIndex
		events = append(events, collEvents...)
		blockTxResults = append(blockTxResults, txResults...)

		interactions[i] = collectionView.Interactions()

		stateView.MergeView(collectionView)
	}

	return &execution.ComputationResult{
		ExecutableBlock:   block,
		StateSnapshots:    interactions,
		Events:            events,
		TransactionResult: blockTxResults,
		GasUsed:           gasUsed,
		StateReads:        stateView.ReadsCount(),
	}, nil
}

func (e *blockComputer) executeCollection(
	ctx context.Context,
	txIndex uint32,
	blockCtx virtualmachine.BlockContext,
	collectionView *delta.View,
	collection *entity.CompleteCollection,
) ([]flow.Event, []flow.TransactionResult, uint32, uint64, error) {

	var colSpan opentracing.Span
	if e.tracer != nil {
		colSpan, _ = e.tracer.StartSpanFromContext(ctx, trace.EXEComputeCollection)
		defer colSpan.Finish()
	}

	var (
		events    []flow.Event
		txResults []flow.TransactionResult
		gasUsed   uint64
	)

	txMetrics := virtualmachine.NewMetricsCollector()

	for _, tx := range collection.Transactions {
		err := func(tx *flow.TransactionBody) error {
			if e.tracer != nil {
				txSpan := e.tracer.StartSpanFromParent(colSpan, trace.EXEComputeTransaction)

				defer func() {
					// Attach runtime metrics to the transaction span.
					//
					// Each duration is the sum of all sub-programs in the transaction.
					//
					// For example, metrics.Parsed() returns the total time spent parsing the transaction itself,
					// as well as any imported programs.
					txSpan.LogFields(
						log.Int64(trace.EXEParseDurationTag, int64(txMetrics.Parsed())),
						log.Int64(trace.EXECheckDurationTag, int64(txMetrics.Checked())),
						log.Int64(trace.EXEInterpretDurationTag, int64(txMetrics.Interpreted())),
					)
					txSpan.Finish()
				}()
			}

			txView := collectionView.NewChild()

			result, err := blockCtx.ExecuteTransaction(txView, tx, virtualmachine.WithMetricsCollector(txMetrics))

			if e.metrics != nil {
				e.metrics.TransactionParsed(txMetrics.Parsed())
				e.metrics.TransactionChecked(txMetrics.Checked())
				e.metrics.TransactionInterpreted(txMetrics.Interpreted())
			}

			if err != nil {
				txIndex++
				return fmt.Errorf("failed to execute transaction: %w", err)
			}

			txEvents, err := virtualmachine.ConvertEvents(txIndex, result)
			txIndex++
			gasUsed += result.GasUsed

			if err != nil {
				return fmt.Errorf("failed to create flow events: %w", err)
			}

			events = append(events, txEvents...)

			txResult := flow.TransactionResult{
				TransactionID: tx.ID(),
			}

			if result.Error != nil {
				txResult.ErrorMessage = result.Error.ErrorMessage()
			}

			txResults = append(txResults, txResult)

			if result.Succeeded() {
				collectionView.MergeView(txView)
			}

			return nil
		}(tx)

		if err != nil {
			return nil, nil, txIndex, 0, err
		}
	}

	return events, txResults, txIndex, gasUsed, nil
}
