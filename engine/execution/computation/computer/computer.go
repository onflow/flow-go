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
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/logging"
)

// VirtualMachine runs procedures
type VirtualMachine interface {
	Run(fvm.Context, fvm.Procedure, state.View, *programs.Programs) error
}

// ViewCommitter commits views's deltas to the ledger and collects the proofs
type ViewCommitter interface {
	// CommitView commits a views' register delta and collects proofs
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
		fvm.WithRestrictedDeployment(false),
		fvm.WithServiceEventCollectionEnabled(),
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

	// check the start state is set
	if !block.HasStartState() {
		return nil, fmt.Errorf("executable block start state is not set")
	}

	blockCtx := fvm.NewContextFromParent(e.vmCtx, fvm.WithBlockHeader(block.Block.Header))
	collections := block.Collections()
	res := &execution.ComputationResult{
		ExecutableBlock:    block,
		Events:             make([]flow.Event, 0),
		ServiceEvents:      make([]flow.Event, 0),
		TransactionResults: make([]flow.TransactionResult, 0),
		StateCommitments:   make([]flow.StateCommitment, 0),
		Proofs:             make([][]byte, 0),
	}

	var txIndex uint32
	var err error
	var wg sync.WaitGroup
	wg.Add(1)

	stateCommitments := make([]flow.StateCommitment, 0, len(collections)+1)
	proofs := make([][]byte, 0, len(collections)+1)

	bc := blockCommitter{
		committer: e.committer,
		blockSpan: blockSpan,
		tracer:    e.tracer,
		state:     *block.StartState,
		views:     make(chan state.View, len(collections)+1),
		callBack: func(state flow.StateCommitment, proof []byte, err error) {
			if err != nil {
				panic(err)
			}
			stateCommitments = append(stateCommitments, state)
			proofs = append(proofs, proof)
		},
	}

	go func() {
		bc.Run()
		wg.Done()
	}()

	for _, collection := range collections {
		colView := stateView.NewChild()
		txIndex, err = e.executeCollection(blockSpan, txIndex, blockCtx, colView, programs, collection, res)
		if err != nil {
			return nil, fmt.Errorf("failed to execute collection: %w", err)
		}
		bc.Commit(colView)
		err = stateView.MergeView(colView)
		if err != nil {
			return nil, fmt.Errorf("cannot merge view: %w", err)
		}
	}

	// executing system chunk
	e.log.Debug().Hex("block_id", logging.Entity(block)).Msg("executing system chunk")
	colView := stateView.NewChild()
	_, err = e.executeSystemCollection(blockSpan, txIndex, colView, programs, res)
	if err != nil {
		return nil, fmt.Errorf("failed to execute system chunk transaction: %w", err)
	}
	bc.Commit(colView)
	err = stateView.MergeView(colView)
	if err != nil {
		return nil, fmt.Errorf("cannot merge view: %w", err)
	}

	// close the views and wait for all views to be committed
	close(bc.views)
	wg.Wait()
	res.StateReads = stateView.(*delta.View).ReadsCount()
	res.StateCommitments = stateCommitments
	res.Proofs = proofs

	return res, nil
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
	tx := blueprints.SystemChunkTransaction(serviceAddress)
	err := e.executeTransaction(tx, colSpan, collectionView, programs, e.systemChunkCtx, txIndex, res)
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

	txCtx := fvm.NewContextFromParent(blockCtx, fvm.WithMetricsReporter(e.metrics), fvm.WithTracer(e.tracer))
	for _, txBody := range collection.Transactions {
		err := e.executeTransaction(txBody, colSpan, collectionView, programs, txCtx, txIndex, res)
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

	txResult := flow.TransactionResult{
		TransactionID:   tx.ID,
		ComputationUsed: tx.ComputationUsed,
	}

	if tx.Err != nil {
		txResult.ErrorMessage = tx.Err.Error()
		e.log.Debug().
			Hex("tx_id", logging.Entity(txBody)).
			Str("error_message", tx.Err.Error()).
			Uint16("error_code", uint16(tx.Err.Code())).
			Msg("transaction execution failed")
	} else {
		e.log.Debug().
			Hex("tx_id", logging.Entity(txBody)).
			Msg("transaction executed successfully")
	}

	mergeSpan := e.tracer.StartSpanFromParent(txSpan, trace.EXEMergeTransactionView)
	defer mergeSpan.Finish()

	// always merge the view, fvm take cares of reverting changes
	// of failed transaction invocation
	err = collectionView.MergeView(txView)
	if err != nil {
		return fmt.Errorf("merging tx view to collection view failed: %w", err)
	}

	res.AddEvents(tx.Events)
	res.AddServiceEvents(tx.ServiceEvents)
	res.AddTransactionResult(&txResult)
	res.AddComputationUsed(tx.ComputationUsed)

	e.log.Info().
		Str("txHash", tx.ID.String()).
		Str("traceID", traceID).
		Int64("timeSpentInMS", time.Since(startedAt).Milliseconds()).
		Msg("transaction executed")
	return nil
}

type blockCommitter struct {
	tracer    module.Tracer
	committer ViewCommitter
	callBack  func(state flow.StateCommitment, proof []byte, err error)
	state     flow.StateCommitment
	views     chan state.View
	blockSpan opentracing.Span
}

func (bc *blockCommitter) Run() {
	for view := range bc.views {
		span := bc.tracer.StartSpanFromParent(bc.blockSpan, trace.EXECommitDelta)
		stateCommit, proof, err := bc.committer.CommitView(view, bc.state)
		bc.callBack(stateCommit, proof, err)
		bc.state = stateCommit
		span.Finish()
	}
}

func (bc *blockCommitter) Commit(view state.View) {
	bc.views <- view
}
