package computer

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	filepath "path/filepath"
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
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/logging"

	"github.com/r3labs/diff/v2"
)

type VirtualMachine interface {
	Run(fvm.Context, fvm.Procedure, state.View, *programs.Programs) error
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
}

type ReadWrites struct {
	Reads  map[string]flow.RegisterID
	Writes map[string]flow.RegisterID
}

func (rw *ReadWrites) Merge(other *ReadWrites) {
	for k, v := range other.Reads {
		rw.Reads[k] = v
	}
	for k, v := range other.Writes {
		rw.Writes[k] = v
	}
}

func (rw *ReadWrites) Conflicts(other *ReadWrites) []flow.RegisterID {

	conflicts := make([]flow.RegisterID, 0)

	for k, v := range other.Reads {
		if _, has := rw.Writes[k]; has {
			conflicts = append(conflicts, v)
		}
	}

	return conflicts
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

	results, err := e.executeBlock(ctx, block, stateView, program)
	if err != nil {
		return nil, fmt.Errorf("failed to execute transactions: %w", err)
	}

	// TODO: compute block fees & reward payments

	return results, nil
}

func printDiff(d diff.Changelog) {

	fmt.Printf("-----DIFF-----\n")
	for _, change := range d {
		fmt.Printf("%s: %s => %s\n", strings.Join(change.Path, "/"), change.From, change.To)
	}
	fmt.Printf("-----/DIFF-----\n")

}

func (e *blockComputer) executeBlock(
	ctx context.Context,
	block *entity.ExecutableBlock,
	stateView state.View,
	programs *programs.Programs,
) (*execution.ComputationResult, error) {

	blockCtx := fvm.NewContextFromParent(e.vmCtx, fvm.WithBlockHeader(block.Block.Header))

	collections := block.Collections()

	var gasUsed uint64

	interactions := make([]*delta.SpockSnapshot, len(collections)+1)

	events := make([]flow.Event, 0)
	serviceEvents := make([]flow.Event, 0)
	blockTxResults := make([]flow.TransactionResult, 0)

	var txIndex uint32

	totalTx := 0
	totalConflictingBlockTxs := 0
	totalConflictingCollectionTxs := 0

	wg := sync.WaitGroup{}

	blockView := stateView.NewChild()
	stateView = stateView.NewChild()

	type CollectionCalculationResults struct {
		event                    []flow.Event
		serviceEvents            []flow.Event
		txResults                []flow.TransactionResult
		nextIndex                uint32
		gas                      uint64
		blockConflicts           []flow.RegisterID
		collectionConflicts      []flow.RegisterID
		txNumber                 int
		conflictingBlockTxs      int
		conflictingCollectionTxs int
		view                     state.View
	}

	collectionData := make([]CollectionCalculationResults, len(collections))

	mutex := sync.Mutex{}

	for i, collection := range collections {

		ii := i
		collectionC := collection

		wg.Add(1)

		func() {

			collectionView := stateView.NewChild()

			freshBlockView := blockView.NewChild()

			collEvents, collServiceEvents, txResults, nextIndex, gas, _, _, txNumber, conflictingBlockTxs, conflictingCollectionTxs, err := e.executeCollection(
				ctx, txIndex, blockCtx, collectionView, freshBlockView, programs, collectionC, block.Block.Header, ii,
			)

			if err != nil {
				panic(fmt.Errorf("failed to execute collection: %w", err))
			}

			//stateInteractions :=  collectionView.(*delta.View).Interactions().Delta
			//alternativeStateInteractions :=  alternativeCollectionView.(*delta.View).Interactions().Delta
			//
			//d, err := diff.Diff(stateInteractions, alternativeStateInteractions)
			//
			//if len(d) > 0 && ii > 0{
			//	//printDiff(d)
			//	fmt.Printf("changeset diff block %d collection %d\n", block.Height(), ii)
			//} else if ii > 0 {
			//	fmt.Printf("all is good for block %d collection %d\n", block.Height(), ii)
			//}

			mutex.Lock()
			defer mutex.Unlock()

			collectionData[ii] = CollectionCalculationResults{
				event:                    collEvents,
				serviceEvents:            collServiceEvents,
				txResults:                txResults,
				nextIndex:                nextIndex,
				gas:                      gas,
				blockConflicts:           nil,
				collectionConflicts:      nil,
				txNumber:                 txNumber,
				conflictingBlockTxs:      conflictingBlockTxs,
				conflictingCollectionTxs: conflictingCollectionTxs,
				view:                     collectionView,
			}

			wg.Done()

			interactions[ii] = collectionView.(*delta.View).Interactions()

			err = stateView.MergeView(collectionView)
			if err != nil {
				panic(fmt.Errorf("cannot merge view: %w", err))
			}
		}()

		totalTx += len(collection.Transactions)
	}

	wg.Wait()

	for _, data := range collectionData {

		//collectionView := stateView.NewChild()
		//
		//e.log.Debug().
		//	Hex("block_id", logging.Entity(block)).
		//	Hex("collection_id", logging.Entity(collection.Guarantee)).
		//	Msg("executing collection")
		//
		//collEvents, collServiceEvents, txResults, nextIndex, gas, _, _, txNumber, conflictingBlockTxs, conflictingCollectionTxs, err := e.executeCollection(
		//	ctx, txIndex, blockCtx, collectionView, programs, collection, blockRW,
		//)
		//if err != nil {
		//	return nil, fmt.Errorf("failed to execute collection: %w", err)
		//}

		gas := data.gas
		nextIndex := data.nextIndex
		collEvents := data.event
		collServiceEvents := data.serviceEvents
		txResults := data.txResults
		//collectionView := data.view
		txNumber := data.txNumber
		conflictingBlockTxs := data.conflictingBlockTxs
		conflictingCollectionTxs := data.conflictingCollectionTxs

		gasUsed += gas

		txIndex = nextIndex
		events = append(events, collEvents...)
		serviceEvents = append(serviceEvents, collServiceEvents...)
		blockTxResults = append(blockTxResults, txResults...)

		//interactions[i] = collectionView.(*delta.View).Interactions()
		//
		//err := stateView.MergeView(collectionView)
		//if err != nil {
		//	return nil, fmt.Errorf("cannot merge view: %w", err)
		//}

		totalTx += txNumber
		totalConflictingBlockTxs += conflictingBlockTxs
		totalConflictingCollectionTxs += conflictingCollectionTxs
	}

	// system chunk
	systemChunkView := stateView.NewChild()
	e.log.Debug().Hex("block_id", logging.Entity(block)).Msg("executing system chunk")

	var colSpan opentracing.Span
	colSpan, _ = e.tracer.StartSpanFromContext(ctx, trace.EXEComputeSystemCollection)
	defer colSpan.Finish()

	serviceAddress := e.vmCtx.Chain.ServiceAddress()

	tx := fvm.SystemChunkTransaction(serviceAddress)

	txMetrics := fvm.NewMetricsCollector()

	txEvents, txServiceEvents, txResult, txGas, err := e.executeTransaction(tx, colSpan, txMetrics, systemChunkView, programs, e.systemChunkCtx, txIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to execute system chunk transaction: %w", err)
	}

	totalTx += 1

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

		CollectionConflicts:      nil,
		BlockConflicts:           nil,
		TotalTx:                  totalTx,
		ConflictingBlockTxs:      totalConflictingBlockTxs,
		ConflictingCollectionTxs: totalConflictingCollectionTxs,
	}, nil
}

func (e *blockComputer) executeCollection(ctx context.Context, txIndex uint32, blockCtx fvm.Context, collectionView state.View, alternativeBlockView state.View, programs *programs.Programs, collection *entity.CompleteCollection, header *flow.Header, collectionIndex int) ([]flow.Event, []flow.Event, []flow.TransactionResult, uint32, uint64, []flow.RegisterID, []flow.RegisterID, int, int, int, error) {

	// call tracing
	startedAt := time.Now()
	var colSpan opentracing.Span
	colSpan, _ = e.tracer.StartSpanFromContext(ctx, trace.EXEComputeCollection)
	defer func() {
		colSpan.SetTag("collection.txCount", len(collection.Transactions))
		colSpan.LogFields(
			log.String("collection.hash", collection.Guarantee.CollectionID.String()),
		)
		colSpan.Finish()
	}()

	var (
		events        []flow.Event
		serviceEvents []flow.Event
		txResults     []flow.TransactionResult
		gasUsed       uint64
	)

	txMetrics := fvm.NewMetricsCollector()

	txCtx := fvm.NewContextFromParent(blockCtx, fvm.WithMetricsCollector(txMetrics), fvm.WithTracer(e.tracer))

	cumulativeBlockConflicts := make([]flow.RegisterID, 0)
	cumulativeCollectionConflicts := make([]flow.RegisterID, 0)

	conflictingBlockTxs := 0
	conflictingCollectionTxs := 0
	totalTx := 0

	baseCollectionView := collectionView.NewChild()

	workingCollectionView := collectionView.NewChild()

	for _, txBody := range collection.Transactions {

		txEvents, txServiceEvents, txResult, txGasUsed, err :=
			e.executeTransaction(txBody, colSpan, txMetrics, workingCollectionView, programs, txCtx, txIndex)
		if err != nil {
			return nil, nil, nil, txIndex, 0, nil, nil, 0, 0, 0, err
		}

		alternativeCollectionView := baseCollectionView.NewChild()

		_, _, _, _, err =
			e.executeTransaction(txBody, colSpan, txMetrics, alternativeCollectionView, programs, txCtx, txIndex)
		if err != nil {
			return nil, nil, nil, txIndex, 0, nil, nil, 0, 0, 0, err
		}

		blockView := alternativeBlockView.NewChild()

		_, _, _, _, err =
			e.executeTransaction(txBody, colSpan, txMetrics, blockView, programs, txCtx, txIndex)
		if err != nil {
			return nil, nil, nil, txIndex, 0, nil, nil, 0, 0, 0, err
		}

		txIndex++
		events = append(events, txEvents...)
		serviceEvents = append(serviceEvents, txServiceEvents...)
		txResults = append(txResults, txResult)
		gasUsed += txGasUsed

		interactions := workingCollectionView.(*delta.View).Interactions().Delta
		collectionInteractions := alternativeCollectionView.(*delta.View).Interactions().Delta
		blockInteractionsInteractions := blockView.(*delta.View).Interactions().Delta

		collectionD, err := diff.Diff(interactions, collectionInteractions)

		if len(collectionD) > 0 {
			conflictingCollectionTxs++
		}

		blockD, err := diff.Diff(interactions, blockInteractionsInteractions)
		if len(blockD) > 0 {
			conflictingBlockTxs++
		}

		writeTxRegisters(txIndex, txBody, header, workingCollectionView, collectionIndex, collection.Collection(), collectionD, blockD)

		totalTx++
	}

	e.log.Info().Str("collectionID", collection.Guarantee.CollectionID.String()).
		Str("blockID", collection.Guarantee.ReferenceBlockID.String()).
		Int("numberOfTransactions", len(collection.Transactions)).
		Int("numberOfEvents", len(events)).
		Int("numberOfServiceEvents", len(serviceEvents)).
		Uint64("totalGasUsed", gasUsed).
		Int64("timeSpentInMS", time.Since(startedAt).Milliseconds()).
		Msg("collection executed")

	collectionView.MergeView(workingCollectionView)

	return events, serviceEvents, txResults, txIndex, gasUsed, cumulativeBlockConflicts, cumulativeCollectionConflicts, totalTx, conflictingBlockTxs, conflictingCollectionTxs, nil
}

type TransactionBody struct {
	ReferenceBlockID flow.Identifier
	Script           string
	Arguments        [][]byte
	GasLimit         uint64
	ProposalKey      flow.ProposalKey
	Payer            flow.Address
	Authorizers      []flow.Address
}

type TxJson struct {
	Transaction         TransactionBody
	Reads               map[string]flow.RegisterID
	Writes              map[string]flow.RegisterID
	CollectionConflicts []string
	BlockConflicts      []string
}

func writeTxRegisters(txIndex uint32, txBody *flow.TransactionBody, header *flow.Header, view state.View, collectionIndex int, collection flow.Collection, collectionD diff.Changelog, blockD diff.Changelog) {

	basedir := "/mnt/data-out/blocks"

	block_path := fmt.Sprintf("%08d_%s", header.Height, header.ID())

	collection_path := fmt.Sprintf("%02d_%s", collectionIndex, collection.ID())

	conflictSuffix := ""

	if len(collectionD) > 0 || len(blockD) > 0 {
		conflictSuffix = "_conflicts"
	}

	tx_path := fmt.Sprintf("%02d_%s%s.json", txIndex-1, txBody.ID(), conflictSuffix)

	dir := filepath.Join(basedir, block_path, collection_path)

	path := filepath.Join(dir, tx_path)

	reads := view.(*delta.View).Reads
	writes := view.(*delta.View).Writes

	s := TxJson{
		Transaction: TransactionBody{
			ReferenceBlockID: txBody.ReferenceBlockID,
			Script:           string(txBody.Script),
			Arguments:        txBody.Arguments,
			GasLimit:         txBody.GasLimit,
			ProposalKey:      txBody.ProposalKey,
			Payer:            txBody.Payer,
			Authorizers:      txBody.Authorizers,
		},
		Reads:               reads,
		Writes:              writes,
		CollectionConflicts: make([]string, len(collectionD)),
		BlockConflicts:      make([]string, len(blockD)),
	}

	for i, change := range collectionD {
		s.CollectionConflicts[i] = strings.Join(change.Path, "/")
	}

	for i, change := range blockD {
		s.BlockConflicts[i] = strings.Join(change.Path, "/")
	}

	data, err := json.MarshalIndent(s, "", "    ")
	if err != nil {
		panic(err)
	}

	err = os.MkdirAll(dir, 0755)
	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile(path, data, 0755)
	if err != nil {
		panic(err)
	}
}

func NewReadWrites() *ReadWrites {
	return &ReadWrites{
		Reads:  map[string]flow.RegisterID{},
		Writes: map[string]flow.RegisterID{},
	}
}

func (e *blockComputer) executeTransaction(
	txBody *flow.TransactionBody,
	colSpan opentracing.Span,
	txMetrics *fvm.MetricsCollector,
	collectionView state.View,
	programs *programs.Programs,
	ctx fvm.Context,
	txIndex uint32,
) ([]flow.Event, []flow.Event, flow.TransactionResult, uint64, error) {

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
			Msg("transaction executed successuint32fully")
	}

	mergeSpan := e.tracer.StartSpanFromParent(txSpan, trace.EXEMergeTransactionView)
	defer mergeSpan.Finish()

	if tx.Err == nil {
		err := collectionView.MergeView(txView)
		if err != nil {
			return nil, nil, txResult, 0, err
		}

	}
	e.log.Info().
		Str("txHash", tx.ID.String()).
		Str("traceID", traceID).
		Int64("timeSpentInMS", time.Since(startedAt).Milliseconds()).
		Msg("transaction executed")

	return tx.Events, tx.ServiceEvents, txResult, tx.GasUsed, nil
}
