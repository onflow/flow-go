package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	_ "net/http/pprof"
	"os"
	"path"
	"reflect"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm"
	led "github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete"
	mtrie2 "github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	wal2 "github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/storage/badger"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	cadenceRuntime "github.com/onflow/cadence/runtime"

	readBadger "github.com/dgraph-io/badger/v2"
)

type Update struct {
	StateCommitment flow.StateCommitment
	Snapshot        *delta.Snapshot
}

type ComputedBlock struct {
	ExecutableBlock *entity.ExecutableBlock
	//Updates         []Update //collectionID -> update
	EndState flow.StateCommitment
	Results  []flow.TransactionResult
}

type Loader struct {
	db      *readBadger.DB
	headers *badger.Headers
	index   *badger.Index
	//identities       *badger.Identities
	guarantees         *badger.Guarantees
	seals              *badger.Seals
	payloads           *badger.Payloads
	commits            *badger.Commits
	transactions       *badger.Transactions
	collections        *badger.Collections
	executionResults   *badger.ExecutionResults
	blocks             *badger.Blocks
	chunkDataPacks     *badger.ChunkDataPacks
	executionState     state.ExecutionState
	metrics            *metrics.NoopCollector
	vm                 *fvm.VirtualMachine
	ctx                context.Context
	mappingMutex       sync.Mutex
	executionDir       string
	dataDir            string
	blockIDsDir        string
	blocksDir          string
	systemChunk        bool
	chain              flow.ChainID
	restricted         bool
	ledger             led.Ledger
	transactionResults *badger.TransactionResults
}

type RootLoadingOnlyWAL struct {
	dir string
}

func (w *RootLoadingOnlyWAL) ReplayOnForest(forest *mtrie2.Forest) error {

	log.Debug().Msg("Loading root.checkpoint")

	filepath := path.Join(w.dir, "root.checkpoint")
	file, err := os.Open(filepath)
	if err != nil {
		log.Fatal().Msgf("cannot open checkpoint file %s: %w", filepath, err)
	}
	defer func() {
		_ = file.Close()
	}()

	flattenedForest, err := wal2.ReadCheckpoint(file)
	if err != nil {
		log.Fatal().Msgf("cannot read checkpoint file %s: %w", filepath, err)
	}

	rebuiltTries, err := flattener.RebuildTries(flattenedForest)
	if err != nil {
		return fmt.Errorf("rebuilding forest from sequenced nodes failed: %w", err)
	}
	err = forest.AddTries(rebuiltTries)

	log.Debug().Msg("Finished loading root.checkpoint")

	return nil
}

func newLoader(protocolDir, executionDir, dataDir, blockIDsDir, blocksDir string, vm *fvm.VirtualMachine, systemChunk bool, chain flow.ChainID, restricted bool) *Loader {
	db := common.InitStorageWithTruncate(protocolDir, true)
	// defer db.Close()

	log.Debug().Msg("initialized protocol storage")

	cacheMetrics := &metrics.NoopCollector{}
	tracer := &trace.NoopTracer{}

	index := badger.NewIndex(cacheMetrics, db)
	guarantees := badger.NewGuarantees(cacheMetrics, db)
	seals := badger.NewSeals(cacheMetrics, db)
	transactions := badger.NewTransactions(cacheMetrics, db)
	headers := badger.NewHeaders(cacheMetrics, db)
	executionResults := badger.NewExecutionResults(cacheMetrics, db)
	executionReceipts := badger.NewExecutionReceipts(cacheMetrics, db, executionResults)

	commits := badger.NewCommits(cacheMetrics, db)
	payloads := badger.NewPayloads(db, index, guarantees, seals, executionReceipts)
	blocks := badger.NewBlocks(db, headers, payloads)
	collections := badger.NewCollections(db, transactions)
	chunkDataPacks := badger.NewChunkDataPacks(db)
	transactionResults := badger.NewTransactionResults(cacheMetrics, db, 100)

	ledger, err := complete.NewLedger(executionDir, 5, cacheMetrics, zerolog.Nop(), nil, complete.DefaultPathFinderVersion)
	if err != nil {
		log.Fatal().Msgf("cannot creat ledger: %w", err)
	}

	executionState := state.NewExecutionState(
		ledger,
		commits,
		blocks,
		headers,
		collections,
		chunkDataPacks,
		executionResults,
		executionReceipts,
		nil,
		nil,
		nil,
		nil,
		db, tracer)

	loader := Loader{
		db:                 db,
		headers:            headers,
		index:              index,
		guarantees:         guarantees,
		seals:              seals,
		payloads:           payloads,
		commits:            commits,
		transactions:       transactions,
		collections:        collections,
		executionResults:   executionResults,
		blocks:             blocks,
		chunkDataPacks:     chunkDataPacks,
		executionState:     executionState,
		metrics:            cacheMetrics,
		vm:                 vm,
		ctx:                context.Background(),
		executionDir:       executionDir,
		dataDir:            dataDir,
		blockIDsDir:        blockIDsDir,
		systemChunk:        systemChunk,
		chain:              chain,
		restricted:         restricted,
		ledger:             ledger,
		transactionResults: transactionResults,
	}
	return &loader
}

var debugStateCommitments = false

func main() {

	// go func() {
	// 	err := http.ListenAndServe(":4000", nil)
	// 	log.Fatal().Err(err).Msg("pprof server error")
	// }()

	totalTx := 0

	mainnet7()

	log.Info().Int("total_transactions", totalTx).Msg("Finished processing mainnet7")

}

func mainnet7() int {

	initialRT := cadenceRuntime.NewInterpreterRuntime()
	vm := fvm.NewVirtualMachine(initialRT)

	log.Debug().Msg("creating loader")
	loader := newLoader("/mnt/data/protocol", "/mnt/data/execution-root", "devnet12", "devnet/block-ids", "devnet/blocks", vm, true, flow.Mainnet, true)

	first := 13_404_174
	last := 13_804_274

	totalTx := 0

	reportCounter := 0
	reportInterval := 100
	startTime := time.Now()

	log.Debug().Msg("starting processing blocks")

	stateCommitment, err := hex.DecodeString("1247d74449a0252ccfe4fd0f8c6dd98e049417b3bffc3554646d92f810e11542")
	if err != nil {
		panic(err)
	}

	totalBlockConflictingTxs := 0
	totalCollectionConflictingTxs := 0

	totalCollectionConflictableTxs := 0
	totalBlockConflictableTxs := 0

	rangeBlockConflictingTxs := 0
	rangeCollectionConflictingTxs := 0

	rangeBlockConflicatbleTxs := 0
	rangeCollectionConflictableTxs := 0

	for i := first; i <= last; i += 1 {
		newStateCommitment, txs, collectionConflictableTxs, blockConflicatableTxs, blockConflictingTxs, collectionConflictingTxs, err := loader.ProcessBlock(i, stateCommitment)

		if err != nil {
			panic(err)
		}

		stateCommitment = newStateCommitment

		totalTx += txs
		totalBlockConflictingTxs += blockConflictingTxs
		totalCollectionConflictingTxs += collectionConflictingTxs

		totalBlockConflictableTxs += blockConflicatableTxs
		totalCollectionConflictableTxs += collectionConflictableTxs

		rangeBlockConflictingTxs += blockConflictingTxs
		rangeCollectionConflictingTxs += collectionConflictingTxs

		rangeBlockConflicatbleTxs += blockConflicatableTxs
		rangeCollectionConflictableTxs += collectionConflictableTxs

		if reportCounter%reportInterval == 0 {
			elapsed := time.Since(startTime)
			log.Info().
				Int("block_height", i).
				Int("block_count", i-first).
				Int("total_txs", totalTx).
				Int("range_collection_conflictable_txs", rangeCollectionConflictableTxs).
				Int("range_collection_conflicting_txs", rangeCollectionConflictingTxs).
				Int("range_block_conflictable_txs", rangeBlockConflicatbleTxs).
				Int("range_block_conflicting_txs", rangeBlockConflictingTxs).
				Int("total_collection_conflectable_txs", totalCollectionConflictableTxs).
				Int("total_collection_conflicting_txs", totalCollectionConflictingTxs).
				Int("total_block_conflictable_txs", totalBlockConflictableTxs).
				Int("total_block_conflicting_txs", totalBlockConflictingTxs).
				Float64("progress", (float64(reportCounter)/float64(last-first))*100).
				Dur("elapsed", elapsed).
				Float64("per_block", float64(elapsed.Milliseconds())/float64(reportInterval)).
				Msg("processed blocks")
			startTime = time.Now()

			rangeBlockConflictingTxs = 0
			rangeCollectionConflictingTxs = 0

			rangeBlockConflicatbleTxs = 0
			rangeCollectionConflictableTxs = 0
		}

		reportCounter++
	}

	return totalTx
}

func (l *Loader) ProcessBlock(i int, stateCommitment flow.StateCommitment) (flow.StateCommitment, int, int, int, int, int, error) {

	totalTx := 0
	collectionConflictableTxs := 0
	blockConflictableTxs := 0

	block, err := l.blocks.ByHeight(uint64(i))
	if err != nil {
		panic(err)
	}

	blockID := block.ID()

	completeCollections := make(map[flow.Identifier]*entity.CompleteCollection, len(block.Payload.Guarantees))

	txResults := make([]flow.TransactionResult, 0)

	for _, collectionGuarantee := range block.Payload.Guarantees {
		collection, err := l.collections.ByID(collectionGuarantee.CollectionID)
		if err != nil {
			log.Fatal().Err(err).Str("block_id", block.ID().String()).Msg("collection not found")
		}
		completeCollections[collectionGuarantee.CollectionID] = &entity.CompleteCollection{
			Guarantee:    collectionGuarantee,
			Transactions: collection.Transactions,
		}
		totalTx += len(collection.Transactions)
		collectionConflictableTxs += len(collection.Transactions) - 1 // to account for first TX in collection which should never has conflict

		for _, tx := range collection.Transactions {
			txResult, err := l.transactionResults.ByBlockIDTransactionID(blockID, tx.ID())
			if err != nil {
				return nil, 0, 0, 0, 0, 0, fmt.Errorf("cannot get transaction result: block %d: %w", block.Header.Height, err)
			}
			txResults = append(txResults, *txResult)
		}
	}

	blockConflictableTxs = totalTx

	if blockConflictableTxs > 0 { // account for empty block to avoid negative values
		blockConflictableTxs--
	}

	executionResult, err := l.executionResults.ByBlockID(blockID)
	if err != nil {
		return nil, 0, 0, 0, 0, 0, fmt.Errorf("cannot find execution results for block %d", block.Header.Height)
	}

	finalStateCommitment, has := executionResult.FinalStateCommitment()
	if !has {
		return nil, 0, 0, 0, 0, 0, fmt.Errorf("cannot find execution final state commitment for block %d", block.Header.Height)
	}

	computedBlock := &ComputedBlock{
		ExecutableBlock: &entity.ExecutableBlock{
			Block:               block,
			CompleteCollections: completeCollections,
			StartState:          stateCommitment,
		},
		EndState: finalStateCommitment,
		Results:  txResults,
	}

	endState, totalBlockConflictsTx, totalCollectionConflictsTx, err := l.executeBlock(computedBlock)
	if err != nil {
		return nil, 0, 0, 0, 0, 0, fmt.Errorf("cannot executed %d: %w", computedBlock.ExecutableBlock.Height(), err)
	}

	return endState, totalTx, collectionConflictableTxs, blockConflictableTxs, totalBlockConflictsTx, totalCollectionConflictsTx, nil
}

func (l *Loader) executeBlock(computedBlock *ComputedBlock) (flow.StateCommitment, int, int, error) {

	vmCtx := fvm.NewContext(
		zerolog.Nop(),
		fvm.WithChain(l.chain.Chain()),
		fvm.WithBlocks(fvm.NewBlockFinder(l.headers)),
		fvm.WithRestrictedAccountCreation(l.restricted),
		fvm.WithRestrictedDeployment(l.restricted),
		fvm.WithAccountStorageLimit(true),
	)

	computationManager, err := computation.New(
		zerolog.Nop(),
		l.metrics,
		trace.NewNoopTracer(),
		nil,
		nil, //module.Local should not be used,
		l.vm,
		vmCtx,
		computation.DefaultProgramsCacheSize,
	)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("cannot create computation manager: %w", err)
	}

	view := l.executionState.NewView(computedBlock.ExecutableBlock.StartState)

	computationResult, err := computationManager.ComputeBlock(
		context.Background(),
		computedBlock.ExecutableBlock,
		view,
	)

	if err != nil {
		return nil, 0, 0, fmt.Errorf("cannot compute block: %w", err)
	}

	var startState flow.StateCommitment = computedBlock.ExecutableBlock.StartState

	for _, snapshot := range computationResult.StateSnapshots {
		endState, err := l.executionState.CommitDelta(context.Background(), snapshot.Delta, startState)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("cannot commit delta for block %d: %w", computedBlock.ExecutableBlock.Height(), err)
		}

		startState = endState
	}

	if !bytes.Equal(computedBlock.EndState, startState) {

		if !compareResults(computedBlock.Results, computationResult.TransactionResult) {

			fmt.Println("DB results")

			spew.Dump(computedBlock.Results)
			fmt.Println("Computer results")
			spew.Dump(computationResult.TransactionResult)

			fmt.Println("---")

			return nil, 0, 0, fmt.Errorf("block %d computed different tx results", computedBlock.ExecutableBlock.Height())
		}

		return nil, 0, 0, fmt.Errorf("block %d computed different state commitment", computedBlock.ExecutableBlock.Height())
	}

	return startState, computationResult.ConflictingBlockTxs, computationResult.ConflictingCollectionTxs, nil
}

func (l *Loader) Close() {
	l.db.Close()
}

func compareResults(a []flow.TransactionResult, b []flow.TransactionResult) bool {
	if len(a) != len(b) {
		return false
	}

	am := make(map[string]string)
	bm := make(map[string]string)

	for _, t := range a {
		am[t.TransactionID.String()] = t.ErrorMessage
	}
	for _, t := range b {
		bm[t.TransactionID.String()] = t.ErrorMessage
	}

	return reflect.DeepEqual(am, bm)
}
