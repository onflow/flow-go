package computer

import (
	"context"
	"fmt"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
	"github.com/rs/zerolog"
	"github.com/uber/jaeger-client-go"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/logging"
)

type VirtualMachine interface {
	Run(fvm.Context, fvm.Procedure, state.View, *fvm.Programs) error
}

// A BlockComputer executes the transactions in a block.
type BlockComputer interface {
	ExecuteBlock(context.Context, *entity.ExecutableBlock, state.View, *fvm.Programs) (*execution.ComputationResult, error)
}

type blockComputer struct {
	vm             VirtualMachine
	vmCtx          fvm.Context
	metrics        module.ExecutionMetrics
	tracer         module.Tracer
	log            zerolog.Logger
	systemChunkCtx fvm.Context
}

// NewBlockComputer creates a new block executor.
func NewBlockComputer(
	vm VirtualMachine,
	vmCtx fvm.Context,
	metrics module.ExecutionMetrics,
	tracer module.Tracer,
	logger zerolog.Logger,
) (BlockComputer, error) {

	systemChunkCtx := fvm.NewContextFromParent(
		vmCtx,
		fvm.WithRestrictedAccountCreation(false),
		fvm.WithRestrictedDeployment(false),
		fvm.WithTransactionProcessors(fvm.NewTransactionInvocator(logger)),
	)

	return &blockComputer{
		vm:             vm,
		vmCtx:          vmCtx,
		metrics:        metrics,
		tracer:         tracer,
		log:            logger,
		systemChunkCtx: systemChunkCtx,
	}, nil
}

// ExecuteBlock executes a block and returns the resulting chunks.
func (e *blockComputer) ExecuteBlock(
	ctx context.Context,
	block *entity.ExecutableBlock,
	stateView state.View,
	program *fvm.Programs,
) (*execution.ComputationResult, error) {

	if e.tracer != nil {
		span, _ := e.tracer.StartSpanFromContext(ctx, trace.EXEComputeBlock)
		defer func() {
			span.SetTag("block.collectioncount", len(block.CompleteCollections))
			span.LogFields(
				log.String("block.hash", block.ID().String()),
			)
			span.Finish()
		}()
	}

	results, err := e.executeBlock(ctx, block, stateView, program)
	if err != nil {
		return nil, fmt.Errorf("failed to execute transactions: %w", err)
	}

	// TODO: compute block fees & reward payments

	return results, nil
}

func (e *blockComputer) executeBlock(
	ctx context.Context,
	block *entity.ExecutableBlock,
	stateView state.View,
	programs *fvm.Programs,
) (*execution.ComputationResult, error) {

	blockCtx := fvm.NewContextFromParent(e.vmCtx, fvm.WithBlockHeader(block.Block.Header))

	collections := block.Collections()

	var gasUsed uint64

	interactions := make([]*delta.SpockSnapshot, len(collections)+1)

	events := make([]flow.Event, 0)
	serviceEvents := make([]flow.Event, 0)
	blockTxResults := make([]flow.TransactionResult, 0)

	var txIndex uint32

	for i, collection := range collections {

		collectionView := stateView.NewChild()

		e.log.Debug().
			Hex("block_id", logging.Entity(block)).
			Hex("collection_id", logging.Entity(collection.Guarantee)).
			Msg("executing collection")

		collEvents, collServiceEvents, txResults, nextIndex, gas, err := e.executeCollection(
			ctx, txIndex, blockCtx, collectionView, programs, collection,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to execute collection: %w", err)
		}

		gasUsed += gas

		txIndex = nextIndex
		events = append(events, collEvents...)
		serviceEvents = append(serviceEvents, collServiceEvents...)
		blockTxResults = append(blockTxResults, txResults...)

		interactions[i] = collectionView.(*delta.View).Interactions()

		err = stateView.MergeView(collectionView)
		if err != nil {
			return nil, err
		}
	}

	// system chunk
	systemChunkView := stateView.NewChild()
	e.log.Debug().Hex("block_id", logging.Entity(block)).Msg("executing system chunk")

	var colSpan opentracing.Span
	if e.tracer != nil {
		colSpan, _ = e.tracer.StartSpanFromContext(ctx, trace.EXEComputeSystemCollection)
		defer colSpan.Finish()
	}

	serviceAddress := e.vmCtx.Chain.ServiceAddress()

	tx := fvm.SystemChunkTransaction(serviceAddress)

	txMetrics := fvm.NewMetricsCollector()

	txEvents, txServiceEvents, txResult, txGas, err := e.executeTransaction(tx, colSpan, txMetrics, systemChunkView, programs, e.systemChunkCtx, txIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to execute system chunk transaction: %w", err)
	}

	events = append(events, txEvents...)
	serviceEvents = append(serviceEvents, txServiceEvents...)
	blockTxResults = append(blockTxResults, txResult)
	gasUsed += txGas
	interactions[len(interactions)-1] = systemChunkView.(*delta.View).Interactions()

	err = stateView.MergeView(systemChunkView)
	if err != nil {
		return nil, err
	}

	return &execution.ComputationResult{
		ExecutableBlock:   block,
		StateSnapshots:    interactions,
		Events:            events,
		ServiceEvents:     serviceEvents,
		TransactionResult: blockTxResults,
		GasUsed:           gasUsed,
		StateReads:        stateView.(*delta.View).ReadsCount(),
	}, nil
}

func (e *blockComputer) executeCollection(
	ctx context.Context,
	txIndex uint32,
	blockCtx fvm.Context,
	collectionView state.View,
	programs *fvm.Programs,
	collection *entity.CompleteCollection,
) ([]flow.Event, []flow.Event, []flow.TransactionResult, uint32, uint64, error) {

	startedAt := time.Now()
	var colSpan opentracing.Span
	if e.tracer != nil {
		colSpan, _ = e.tracer.StartSpanFromContext(ctx, trace.EXEComputeCollection)
		defer func() {
			colSpan.SetTag("collection.txcount", len(collection.Transactions))
			colSpan.LogFields(
				log.String("collection.hash", collection.Guarantee.CollectionID.String()),
			)
			colSpan.Finish()
		}()
	}

	var (
		events        []flow.Event
		serviceEvents []flow.Event
		txResults     []flow.TransactionResult
		gasUsed       uint64
	)

	txMetrics := fvm.NewMetricsCollector()

	txCtx := fvm.NewContextFromParent(blockCtx, fvm.WithMetricsCollector(txMetrics), fvm.WithTracer(e.tracer))

	for _, txBody := range collection.Transactions {

		txEvents, txServiceEvents, txResult, txGasUsed, err :=
			e.executeTransaction(txBody, colSpan, txMetrics, collectionView, programs, txCtx, txIndex)

		txIndex++
		events = append(events, txEvents...)
		serviceEvents = append(serviceEvents, txServiceEvents...)
		txResults = append(txResults, txResult)
		gasUsed += txGasUsed

		if err != nil {
			return nil, nil, nil, txIndex, 0, err
		}
	}

	e.log.Info().Str("collectionID", collection.Guarantee.CollectionID.String()).
		Str("blockID", collection.Guarantee.ReferenceBlockID.String()).
		Int("numberOfTransactions", len(collection.Transactions)).
		Int("numberOfEvents", len(events)).
		Int("numberOfServiceEvents", len(serviceEvents)).
		Uint64("totalGasUsed", gasUsed).
		Int64("timeSpentInMS", time.Since(startedAt).Milliseconds()).
		Msg("collection executed")

	return events, serviceEvents, txResults, txIndex, gasUsed, nil
}

func (e *blockComputer) executeTransaction(
	txBody *flow.TransactionBody,
	colSpan opentracing.Span,
	txMetrics *fvm.MetricsCollector,
	collectionView state.View,
	programs *fvm.Programs,
	ctx fvm.Context,
	txIndex uint32,
) ([]flow.Event, []flow.Event, flow.TransactionResult, uint64, error) {

	var txSpan opentracing.Span
	var traceID string

	if e.tracer != nil {
		txSpan = e.tracer.StartSpanFromParent(colSpan, trace.EXEComputeTransaction)

		if sc, ok := txSpan.Context().(jaeger.SpanContext); ok {
			traceID = sc.TraceID().String()
		}

		defer func() {
			// Attach runtime metrics to the transaction span.
			//
			// Each duration is the sum of all sub-programs in the transaction.
			//
			// For example, metrics.Parsed() returns the total time spent parsing the transaction itself,
			// as well as any imported programs.
			txSpan.SetTag("transaction.proposer", txBody.ProposalKey.Address.String())
			txSpan.SetTag("transaction.payer", txBody.Payer.String())
			txSpan.LogFields(
				log.String("transaction.ID", txBody.ID().String()),
				log.Int64(trace.EXEParseDurationTag, int64(txMetrics.Parsed())),
				log.Int64(trace.EXECheckDurationTag, int64(txMetrics.Checked())),
				log.Int64(trace.EXEInterpretDurationTag, int64(txMetrics.Interpreted())),
				log.Int64(trace.EXEValueEncodingDurationTag, int64(txMetrics.ValueEncoded())),
				log.Int64(trace.EXEValueDecodingDurationTag, int64(txMetrics.ValueDecoded())),
			)
			txSpan.Finish()
		}()
	}

	e.log.Debug().
		Hex("tx_id", logging.Entity(txBody)).
		Msg("executing transaction")

	txView := collectionView.NewChild()

	tx := fvm.Transaction(txBody, txIndex)
	tx.SetTraceSpan(txSpan)

	err := e.vm.Run(ctx, tx, txView, programs)

	if e.metrics != nil {
		e.metrics.TransactionParsed(txMetrics.Parsed())
		e.metrics.TransactionChecked(txMetrics.Checked())
		e.metrics.TransactionInterpreted(txMetrics.Interpreted())
	}

	if err != nil {
		return nil, nil, flow.TransactionResult{}, 0, fmt.Errorf("failed to execute transaction: %w", err)
	}

	txResult := flow.TransactionResult{
		TransactionID: tx.ID,
	}

	if tx.Err != nil {
		txResult.ErrorMessage = tx.Err.Error()
		e.log.Debug().
			Hex("tx_id", logging.Entity(txBody)).
			Str("error_message", tx.Err.Error()).
			Uint32("error_code", tx.Err.Code()).
			Msg("transaction execution failed")
	} else {
		e.log.Debug().
			Hex("tx_id", logging.Entity(txBody)).
			Msg("transaction executed successfully")
	}

	if tx.Err == nil {
		err := collectionView.MergeView(txView)
		if err != nil {
			return nil, nil, txResult, 0, err
		}

	}
	e.log.Info().
		Str("txHash", tx.ID.String()).
		Str("traceID", traceID).
		Msg("transaction executed")

	return tx.Events, tx.ServiceEvents, txResult, tx.GasUsed, nil
}
