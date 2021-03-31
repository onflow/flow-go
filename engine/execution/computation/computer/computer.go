package computer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
	"github.com/rs/zerolog"
	"github.com/uber/jaeger-client-go"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/logging"
)

type VirtualMachine interface {
	Run(fvm.Context, fvm.Procedure, state.View, *programs.Programs) error
}

type ViewCommitter interface {
	// CommitView commits a views' register delta and returns a new state commitment and proof.
	CommitView(state.View, flow.StateCommitment) (flow.StateCommitment, []byte, error)
}

// A BlockComputer executes the transactions in a block.
type BlockComputer interface {
	ExecuteBlock(context.Context, *entity.ExecutableBlock, state.View, *programs.Programs) (*execution.ComputationResult, error)
}

type blockComputer struct {
	vm             VirtualMachine
	vmCtx          fvm.Context
	metrics        module.ExecutionMetrics
	tracer         module.Tracer
	log            zerolog.Logger
	systemChunkCtx fvm.Context
	committer      ViewCommitter
}

// NewBlockComputer creates a new block executor.
func NewBlockComputer(
	vm VirtualMachine,
	vmCtx fvm.Context,
	metrics module.ExecutionMetrics,
	tracer module.Tracer,
	logger zerolog.Logger,
	committer ViewCommitter,
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
		committer:      committer,
	}, nil
}

// ExecuteBlock executes a block and returns the resulting chunks.
func (e *blockComputer) ExecuteBlock(
	ctx context.Context,
	block *entity.ExecutableBlock,
	stateView state.View,
	program *programs.Programs,
) (*execution.ComputationResult, error) {

	// call tracer
	span, _ := e.tracer.StartSpanFromContext(ctx, trace.EXEComputeBlock)
	defer func() {
		span.SetTag("block.collectioncount", len(block.CompleteCollections))
		span.LogFields(
			log.String("block.hash", block.ID().String()),
		)
		span.Finish()
	}()

	results, err := e.executeBlock(span, block, stateView, program)
	if err != nil {
		return nil, fmt.Errorf("failed to execute transactions: %w", err)
	}

	// TODO: compute block fees & reward payments

	return results, nil
}

func (e *blockComputer) executeBlock(
	blockSpan opentracing.Span,
	block *entity.ExecutableBlock,
	stateView state.View,
	programs *programs.Programs,
) (*execution.ComputationResult, error) {

	blockCtx := fvm.NewContextFromParent(e.vmCtx, fvm.WithBlockHeader(block.Block.Header))
	collections := block.Collections()
	res := &execution.ComputationResult{
		ExecutableBlock:    block,
		Events:             make([]flow.Event, 0),
		ServiceEvents:      make([]flow.Event, 0),
		TransactionResults: make([]flow.TransactionResult, 0),
	}

	var txIndex uint32
	var err error
	var err1 error
	var err2 error
	var colView state.View
	var prevColView state.View
	var stateCommit flow.StateCommitment

	stateCommit = block.StartState

	if len(collections) > 0 {

		var wg sync.WaitGroup

		// execute the first collection
		colView = stateView.NewChild()
		txIndex, err1 = e.executeCollection(blockSpan, txIndex, blockCtx, colView, programs, collections[0], res)
		if err1 != nil {
			return nil, fmt.Errorf("failed to execute collection: %w", err1)
		}
		prevColView = colView
		err1 = stateView.MergeView(colView)
		if err1 != nil {
			return nil, fmt.Errorf("cannot merge view: %w", err1)
		}

		// executing collections
		for _, collection := range collections[1:] {
			wg.Add(2)
			colView = stateView.NewChild()

			go func() {
				txIndex, err1 = e.executeCollection(blockSpan, txIndex, blockCtx, colView, programs, collection, res)
				wg.Done()
			}()

			go func() {
				stateCommit, err2 = e.commitView(blockSpan, prevColView, stateCommit, res)
				wg.Done()
			}()

			wg.Wait()
			if err1 != nil {
				return nil, fmt.Errorf("failed to execute collection: %w", err1)
			}
			if err2 != nil {
				return nil, fmt.Errorf("failed to commit view while executing a collection: %w", err2)
			}
			prevColView = colView
			err = stateView.MergeView(colView)
			if err != nil {
				return nil, fmt.Errorf("cannot merge view: %w", err)
			}
		}

		// commit last view
		stateCommit, err2 = e.commitView(blockSpan, prevColView, stateCommit, res)
		if err2 != nil {
			return nil, fmt.Errorf("failed to commit view while executing a collection: %w", err2)
		}
	}

	// TODO make this part also parallel (right now it doesn't worth the time)
	// executing system chunk
	e.log.Debug().Hex("block_id", logging.Entity(block)).Msg("executing system chunk")
	collectionView := stateView.NewChild()
	_, err = e.executeSystemCollection(blockSpan, txIndex, collectionView, programs, res)
	if err != nil {
		return nil, fmt.Errorf("failed to execute system chunk transaction: %w", err)
	}
	stateCommit, err = e.commitView(blockSpan, collectionView, stateCommit, res)
	err = stateView.MergeView(collectionView)
	if err != nil {
		return nil, fmt.Errorf("cannot merge view: %w", err)
	}
	res.StateReads = stateView.(*delta.View).ReadsCount()
	return res, nil
}

func (e *blockComputer) commitView(
	blockSpan opentracing.Span,
	collectionView state.View,
	state flow.StateCommitment,
	res *execution.ComputationResult,
) (flow.StateCommitment, error) {
	span := e.tracer.StartSpanFromParent(blockSpan, trace.EXECommitDelta)
	defer span.Finish()

	stateCommit, proof, err := e.committer.CommitView(collectionView, state)
	res.AddStateCommitment(stateCommit)
	res.AddProof(proof)
	return stateCommit, err
}

func (e *blockComputer) executeSystemCollection(
	blockSpan opentracing.Span,
	txIndex uint32,
	collectionView state.View,
	programs *programs.Programs,
	res *execution.ComputationResult,
) (uint32, error) {

	colSpan := e.tracer.StartSpanFromParent(blockSpan, trace.EXEComputeSystemCollection)
	defer colSpan.Finish()

	serviceAddress := e.vmCtx.Chain.ServiceAddress()
	tx := fvm.SystemChunkTransaction(serviceAddress)
	txMetrics := fvm.NewMetricsCollector()
	err := e.executeTransaction(tx, colSpan, txMetrics, collectionView, programs, e.systemChunkCtx, txIndex, res)
	txIndex++
	if err != nil {
		return txIndex, err
	}
	res.AddStateSnapshot(collectionView.(*delta.View).Interactions())
	return txIndex, err
}

func (e *blockComputer) executeCollection(
	blockSpan opentracing.Span,
	txIndex uint32,
	blockCtx fvm.Context,
	collectionView state.View,
	programs *programs.Programs,
	collection *entity.CompleteCollection,
	res *execution.ComputationResult,
) (uint32, error) {

	e.log.Debug().
		Hex("block_id", logging.Entity(blockCtx.BlockHeader)).
		Hex("collection_id", logging.Entity(collection.Guarantee)).
		Msg("executing collection")

	// call tracing
	startedAt := time.Now()
	colSpan := e.tracer.StartSpanFromParent(blockSpan, trace.EXEComputeCollection)
	defer func() {
		colSpan.SetTag("collection.txCount", len(collection.Transactions))
		colSpan.LogFields(
			log.String("collection.hash", collection.Guarantee.CollectionID.String()),
		)
		colSpan.Finish()
	}()

	txMetrics := fvm.NewMetricsCollector()
	txCtx := fvm.NewContextFromParent(blockCtx, fvm.WithMetricsCollector(txMetrics), fvm.WithTracer(e.tracer))
	for _, txBody := range collection.Transactions {
		err := e.executeTransaction(txBody, colSpan, txMetrics, collectionView, programs, txCtx, txIndex, res)
		txIndex++
		if err != nil {
			return txIndex, err
		}
	}
	res.AddStateSnapshot(collectionView.(*delta.View).Interactions())
	e.log.Info().Str("collectionID", collection.Guarantee.CollectionID.String()).
		Str("blockID", collection.Guarantee.ReferenceBlockID.String()).
		Int("numberOfTransactions", len(collection.Transactions)).
		Int64("timeSpentInMS", time.Since(startedAt).Milliseconds()).
		Msg("collection executed")

	return txIndex, nil
}

func (e *blockComputer) executeTransaction(
	txBody *flow.TransactionBody,
	colSpan opentracing.Span,
	txMetrics *fvm.MetricsCollector,
	collectionView state.View,
	programs *programs.Programs,
	ctx fvm.Context,
	txIndex uint32,
	res *execution.ComputationResult,
) error {

	startedAt := time.Now()
	var txSpan opentracing.Span
	var traceID string
	// call tracing
	txSpan = e.tracer.StartSpanFromParent(colSpan, trace.EXEComputeTransaction)

	if sc, ok := txSpan.Context().(jaeger.SpanContext); ok {
		traceID = sc.TraceID().String()
	}

	defer func() {
		txSpan.LogFields(
			log.String("transaction.ID", txBody.ID().String()),
		)
		txSpan.Finish()
	}()

	e.log.Debug().
		Hex("tx_id", logging.Entity(txBody)).
		Msg("executing transaction")

	txView := collectionView.NewChild()

	tx := fvm.Transaction(txBody, txIndex)
	tx.SetTraceSpan(txSpan)

	err := e.vm.Run(ctx, tx, txView, programs)
	if err != nil {
		return fmt.Errorf("failed to execute transaction: %w", err)
	}

	if e.metrics != nil {
		e.metrics.TransactionParsed(txMetrics.Parsed())
		e.metrics.TransactionChecked(txMetrics.Checked())
		e.metrics.TransactionInterpreted(txMetrics.Interpreted())
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

	mergeSpan := e.tracer.StartSpanFromParent(txSpan, trace.EXEMergeTransactionView)
	defer mergeSpan.Finish()

	if tx.Err == nil {
		err := collectionView.MergeView(txView)
		if err != nil {
			return err
		}

	}

	res.AddEvents(tx.Events)
	res.AddServiceEvents(tx.ServiceEvents)
	res.AddTransactionResult(&txResult)
	res.AddGasUsed(tx.GasUsed)

	e.log.Info().
		Str("txHash", tx.ID.String()).
		Str("traceID", traceID).
		Int64("timeSpentInMS", time.Since(startedAt).Milliseconds()).
		Msg("transaction executed")
	return nil
}
