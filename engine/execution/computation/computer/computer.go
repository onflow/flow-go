package computer

import (
	"context"
	"fmt"
	"strings"
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
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/logging"
)

const SystemChunkEventCollectionMaxSize = 256_000_000  // ~256MB
const SystemChunkLedgerIntractionLimit = 1_000_000_000 // ~1GB
const MaxTransactionErrorStringSize = 1000             // 1000 chars

// VirtualMachine runs procedures
type VirtualMachine interface {
	Run(fvm.Context, fvm.Procedure, state.View, *programs.Programs) error
}

// ViewCommitter commits views's deltas to the ledger and collects the proofs
type ViewCommitter interface {
	// CommitView commits a views' register delta and collects proofs
	CommitView(state.View, flow.StateCommitment) (flow.StateCommitment, []byte, *ledger.TrieUpdate, error)
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

func SystemChunkContext(vmCtx fvm.Context, logger zerolog.Logger) fvm.Context {
	return fvm.NewContextFromParent(
		vmCtx,
		fvm.WithRestrictedDeployment(false),
		fvm.WithTransactionFeesEnabled(false),
		fvm.WithServiceEventCollectionEnabled(),
		fvm.WithTransactionProcessors(fvm.NewTransactionInvoker(logger)),
		fvm.WithMaxStateInteractionSize(SystemChunkLedgerIntractionLimit),
		fvm.WithEventCollectionSizeLimit(SystemChunkEventCollectionMaxSize),
	)
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
	return &blockComputer{
		vm:             vm,
		vmCtx:          vmCtx,
		metrics:        metrics,
		tracer:         tracer,
		log:            logger,
		systemChunkCtx: SystemChunkContext(vmCtx, logger),
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

	span, _, isSampled := e.tracer.StartBlockSpan(ctx, block.ID(), trace.EXEComputeBlock)
	if isSampled {
		span.LogFields(log.Int("collection_counts", len(block.CompleteCollections)))
	}
	defer span.Finish()

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
	systemChunkCtx := fvm.NewContextFromParent(e.systemChunkCtx, fvm.WithBlockHeader(block.Block.Header))
	collections := block.Collections()

	chunksSize := len(collections) + 1 // + 1 system chunk
	txCountTotal := 0
	for _, c := range block.CompleteCollections {
		txCountTotal += len(c.Transactions)
	}
	txCountTotal += 1 // system transaction

	res := &execution.ComputationResult{
		ExecutableBlock:    block,
		Events:             make([]flow.EventsList, chunksSize),
		EventsHashes:       make([]flow.Identifier, chunksSize),
		ServiceEvents:      make(flow.EventsList, 0),
		TransactionResults: make([]flow.TransactionResult, txCountTotal),
		StateCommitments:   make([]flow.StateCommitment, 0),
		Proofs:             make([][]byte, 0),
	}

	var txIndex uint32
	var err error
	var wg sync.WaitGroup
	wg.Add(2) // block committer and event hasher

	var executionWaitGroup sync.WaitGroup
	executionWaitGroup.Add(len(collections))

	stateCommitments := make([]flow.StateCommitment, 0, len(collections)+1)
	proofs := make([][]byte, 0, len(collections)+1)
	trieUpdates := make([]*ledger.TrieUpdate, 0, len(collections)+1)

	bc := blockCommitter{
		committer: e.committer,
		blockSpan: blockSpan,
		tracer:    e.tracer,
		state:     *block.StartState,
		views:     make(chan state.View, len(collections)+1),
		callBack: func(state flow.StateCommitment, proof []byte, trieUpdate *ledger.TrieUpdate, err error) {
			if err != nil {
				panic(err)
			}
			stateCommitments = append(stateCommitments, state)
			proofs = append(proofs, proof)
			trieUpdates = append(trieUpdates, trieUpdate)
		},
	}

	eh := eventHasher{
		tracer:    e.tracer,
		data:      make(chan eventsWithIndex, len(collections)+1),
		blockSpan: blockSpan,
		callBack: func(hash flow.Identifier, index int, err error) {
			if err != nil {
				panic(err)
			}
			res.EventsHashes[index] = hash
		},
	}

	go func() {
		bc.Run()
		wg.Done()
	}()

	go func() {
		eh.Run()
		wg.Done()
	}()

	cResolve := make(chan *execution.ComputationResult, len(collections))
	resolveDone := make(chan bool, 1)
	go func() {
		resolve(cResolve, res)
		resolveDone <- true
	}()

	collectionIndex := 0

	for _, collection := range collections {
		collectionView := stateView.NewChild()
		collectionResult := &execution.ComputationResult{
			ExecutableBlock:    block,
			Events:             make([]flow.EventsList, chunksSize),
			ServiceEvents:      make(flow.EventsList, 0),
			TransactionResults: make([]flow.TransactionResult, txCountTotal),
			StateCommitments:   make([]flow.StateCommitment, 0),
			Proofs:             make([][]byte, 0),
		}
		go func(collectionIndex int, txIndex uint32, collection *entity.CompleteCollection, collectionResult *execution.ComputationResult, collectionView state.View) {
			e.executeCollection(blockSpan, collectionIndex, txIndex, blockCtx, collectionView, programs, collection, collectionResult, cResolve)
			bc.Commit(collectionView)
			eh.Hash(collectionResult.Events[collectionIndex], collectionIndex)
			stateView.MergeView(collectionView)

			executionWaitGroup.Done()
		}(collectionIndex, txIndex, collection, collectionResult, collectionView)

		collectionIndex++
		txIndex += uint32(len(collection.Transactions))
	}
	// wait for all collections to finish executing and resolve
	executionWaitGroup.Wait()
	close(cResolve)
	// block on Resolve()
	<-resolveDone

	// executing system chunk
	e.log.Debug().Hex("block_id", logging.Entity(block)).Msg("executing system chunk")
	colView := stateView.NewChild()
	_, err = e.executeSystemCollection(blockSpan, collectionIndex, txIndex, systemChunkCtx, colView, programs, res)
	if err != nil {
		return nil, fmt.Errorf("failed to execute system chunk transaction: %w", err)
	}
	bc.Commit(colView)
	eh.Hash(res.Events[len(res.Events)-1], collectionIndex)
	err = stateView.MergeView(colView)
	if err != nil {
		return nil, fmt.Errorf("cannot merge view: %w", err)
	}

	// close the views and wait for all views to be committed
	close(bc.views)
	close(eh.data)
	wg.Wait()
	res.StateReads = stateView.(*delta.View).ReadsCount()
	res.StateCommitments = stateCommitments
	res.Proofs = proofs
	res.TrieUpdates = trieUpdates

	return res, nil
}

// Take execution results from collections executing in parallel and add all results to block result
func resolve(cResolve chan *execution.ComputationResult, res *execution.ComputationResult) {
	for collectionResult := range cResolve {
		for _, snapshot := range collectionResult.StateSnapshots {
			res.AddStateSnapshot(snapshot)
		}
		for i, events := range collectionResult.Events {
			if events != nil {
				res.AddEvents(i, events)
			}
		}
		res.AddServiceEvents(collectionResult.ServiceEvents)
		for j, txResult := range collectionResult.TransactionResults {
			if (txResult != flow.TransactionResult{}) {
				res.TransactionResults[j] = txResult
			}
		}
		res.ComputationUsed += collectionResult.ComputationUsed
	}
}

func (e *blockComputer) executeSystemCollection(
	blockSpan opentracing.Span,
	collectionIndex int,
	txIndex uint32,
	systemChunkCtx fvm.Context,
	collectionView state.View,
	programs *programs.Programs,
	res *execution.ComputationResult,
) (uint32, error) {

	colSpan := e.tracer.StartSpanFromParent(blockSpan, trace.EXEComputeSystemCollection)
	defer colSpan.Finish()

	tx, err := blueprints.SystemChunkTransaction(e.vmCtx.Chain)
	if err != nil {
		return txIndex, fmt.Errorf("could not get system chunk transaction: %w", err)
	}

	err = e.executeTransaction(tx, colSpan, collectionView, programs, systemChunkCtx, collectionIndex, txIndex, res, true)
	txIndex++

	if err != nil {
		return txIndex, err
	}

	systemChunkTxResult := res.TransactionResults[len(res.TransactionResults)-1]
	if systemChunkTxResult.ErrorMessage != "" {
		// This log is used as the data source for an alert on grafana.
		// The system_chunk_error field must not be changed without adding the corresponding
		// changes in grafana. https://github.com/dapperlabs/flow-internal/issues/1546
		e.log.Error().
			Str("error_message", systemChunkTxResult.ErrorMessage).
			Hex("block_id", logging.Entity(systemChunkCtx.BlockHeader)).
			Bool("system_chunk_error", true).
			Bool("critical_error", true).
			Msg("error executing system chunk transaction")
	}

	res.AddStateSnapshot(collectionView.(*delta.View).Interactions())

	return txIndex, err
}

func (e *blockComputer) executeCollection(
	blockSpan opentracing.Span,
	collectionIndex int,
	txIndex uint32,
	blockCtx fvm.Context,
	collectionView state.View,
	programs *programs.Programs,
	collection *entity.CompleteCollection,
	res *execution.ComputationResult,
	cResolve chan *execution.ComputationResult,
) (uint32, error) {

	e.log.Debug().
		Hex("block_id", logging.Entity(blockCtx.BlockHeader)).
		Hex("collection_id", logging.Entity(collection.Guarantee)).
		Msg("executing collection")

	// call tracing
	startedAt := time.Now()
	computationUsedUpToNow := res.ComputationUsed
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
		err := e.executeTransaction(txBody, colSpan, collectionView, programs, txCtx, collectionIndex, txIndex, res, false)
		txIndex++
		if err != nil {
			return txIndex, err
		}
	}
	res.AddStateSnapshot(collectionView.(*delta.View).Interactions())
	e.log.Info().Str("collectionID", collection.Guarantee.CollectionID.String()).
		Str("referenceBlockID", collection.Guarantee.ReferenceBlockID.String()).
		Hex("blockID", logging.Entity(blockCtx.BlockHeader)).
		Int("numberOfTransactions", len(collection.Transactions)).
		Int64("timeSpentInMS", time.Since(startedAt).Milliseconds()).
		Msg("collection executed")

	e.metrics.ExecutionCollectionExecuted(time.Since(startedAt), res.ComputationUsed-computationUsedUpToNow, len(collection.Transactions))

	// send computation result to resolver channel
	cResolve <- res
	return txIndex, nil
}

func (e *blockComputer) executeTransaction(
	txBody *flow.TransactionBody,
	colSpan opentracing.Span,
	collectionView state.View,
	programs *programs.Programs,
	ctx fvm.Context,
	collectionIndex int,
	txIndex uint32,
	res *execution.ComputationResult,
	isSystemChunk bool,
) error {
	startedAt := time.Now()
	txID := txBody.ID()

	// we capture two spans one for tx-based view and one for the current context (block-based) view
	txSpan := e.tracer.StartSpanFromParent(colSpan, trace.EXEComputeTransaction)
	txSpan.LogFields(log.String("tx_id", txID.String()))
	txSpan.LogFields(log.Uint32("tx_index", txIndex))
	txSpan.LogFields(log.Int("col_index", collectionIndex))
	defer txSpan.Finish()

	var traceID string
	txInternalSpan, _, isSampled := e.tracer.StartTransactionSpan(context.Background(), txID, trace.EXERunTransaction)
	if isSampled {
		txInternalSpan.LogFields(log.String("tx_id", txID.String()))
		if sc, ok := txInternalSpan.Context().(jaeger.SpanContext); ok {
			traceID = sc.TraceID().String()
		}
	}
	defer txInternalSpan.Finish()

	e.log.Info().
		Str("tx_id", txID.String()).
		Uint32("tx_index", txIndex).
		Str("block_id", res.ExecutableBlock.ID().String()).
		Uint64("height", res.ExecutableBlock.Block.Header.Height).
		Bool("system_chunk", isSystemChunk).
		Msg("executing transaction in fvm")

	tx := fvm.Transaction(txBody, txIndex)
	if isSampled {
		tx.SetTraceSpan(txInternalSpan)
	}

	txView := collectionView.NewChild()
	err := e.vm.Run(ctx, tx, txView, programs)
	if err != nil {
		return fmt.Errorf("failed to execute transaction %v for block %v at height %v: %w",
			txID.String(),
			res.ExecutableBlock.ID(),
			res.ExecutableBlock.Block.Header.Height,
			err)
	}

	txResult := flow.TransactionResult{
		TransactionID:   tx.ID,
		ComputationUsed: tx.ComputationUsed,
	}

	if tx.Err != nil {
		// limit the size of transaction error that is going to be captured
		errorMsg := tx.Err.Error()
		if len(errorMsg) > MaxTransactionErrorStringSize {
			split := int(MaxTransactionErrorStringSize/2) - 1
			var sb strings.Builder
			sb.WriteString(errorMsg[:split])
			sb.WriteString(" ... ")
			sb.WriteString(errorMsg[len(errorMsg)-split:])
			errorMsg = sb.String()
		}
		txResult.ErrorMessage = errorMsg
	}

	mergeSpan := e.tracer.StartSpanFromParent(txSpan, trace.EXEMergeTransactionView)
	defer mergeSpan.Finish()

	// always merge the view, fvm take cares of reverting changes
	// of failed transaction invocation
	err = collectionView.MergeView(txView)
	if err != nil {
		return fmt.Errorf("merging tx view to collection view failed for tx %v: %w",
			txID.String(), err)
	}
	res.AddEvents(collectionIndex, tx.Events)
	res.AddServiceEvents(tx.ServiceEvents)
	res.AddIndexedTransactionResult(&txResult, int(txIndex))
	res.AddComputationUsed(tx.ComputationUsed)

	lg := e.log.With().
		Hex("tx_id", txResult.TransactionID[:]).
		Str("block_id", res.ExecutableBlock.ID().String()).
		Str("traceID", traceID).
		Uint64("computation_used", txResult.ComputationUsed).
		Int64("timeSpentInMS", time.Since(startedAt).Milliseconds()).
		Logger()

	if tx.Err != nil {
		lg.Info().
			Str("error_message", txResult.ErrorMessage).
			Uint16("error_code", uint16(tx.Err.Code())).
			Msg("transaction executed failed")
	} else {
		lg.Info().Msg("transaction executed successfully")
	}

	e.metrics.ExecutionTransactionExecuted(time.Since(startedAt), tx.ComputationUsed, len(tx.Events), tx.Err != nil)
	return nil
}

type blockCommitter struct {
	tracer    module.Tracer
	committer ViewCommitter
	callBack  func(state flow.StateCommitment, proof []byte, update *ledger.TrieUpdate, err error)
	state     flow.StateCommitment
	views     chan state.View
	blockSpan opentracing.Span
}

func (bc *blockCommitter) Run() {
	for view := range bc.views {
		span := bc.tracer.StartSpanFromParent(bc.blockSpan, trace.EXECommitDelta)
		stateCommit, proof, trieUpdate, err := bc.committer.CommitView(view, bc.state)
		bc.callBack(stateCommit, proof, trieUpdate, err)
		bc.state = stateCommit
		span.Finish()
	}
}

func (bc *blockCommitter) Commit(view state.View) {
	bc.views <- view
}

type eventHasher struct {
	tracer    module.Tracer
	callBack  func(hash flow.Identifier, index int, err error)
	data      chan eventsWithIndex
	blockSpan opentracing.Span
}

func (eh *eventHasher) Run() {
	for data := range eh.data {
		events := data.events
		index := data.index
		span := eh.tracer.StartSpanFromParent(eh.blockSpan, trace.EXEHashEvents)
		rootHash, err := flow.EventsMerkleRootHash(events)
		eh.callBack(rootHash, index, err)
		span.Finish()
	}
}

// this type is a bit awkward/thrown-together
type eventsWithIndex struct {
	events flow.EventsList
	index  int
}

func (eh *eventHasher) Hash(events flow.EventsList, index int) {
	eh.data <- eventsWithIndex{events, index}
}
