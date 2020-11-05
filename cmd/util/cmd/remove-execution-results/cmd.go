package remove

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/engine/execution/state"
	ledger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol"
	protocolbadger "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/storage"
	storagebadger "github.com/onflow/flow-go/storage/badger"
)

var (
	flagFromHeight        uint64
	flagDataDir           string
	flagExecutionStateDir string
)

var Cmd = &cobra.Command{
	Use:   "remove-execution-results",
	Short: "Remove execution results for blocks above a certain height",
	Run:   run,
}

func init() {

	Cmd.Flags().Uint64Var(&flagFromHeight, "from-height", 0,
		"the height of the block to remove execution results from (inclusive). Must be a finalized height")
	_ = Cmd.MarkFlagRequired("from-height")

	Cmd.Flags().StringVar(&flagDataDir, "datadir", "",
		"directory that stores the protocol state")
	_ = Cmd.MarkFlagRequired("datadir")

	Cmd.Flags().StringVar(&flagExecutionStateDir, "execution-state-dir", "",
		"Execution Node state dir (where WAL logs are written")
	_ = Cmd.MarkFlagRequired("execution-state-dir")
}

func run(*cobra.Command, []string) {
	log.Info().
		Str("datadir", flagDataDir).
		Str("executiondir", flagExecutionStateDir).
		Uint64("from-height", flagFromHeight).
		Msg("flags")

	db := common.InitStorage(flagDataDir)

	protoState, execState, err := initStates(db, flagExecutionStateDir)
	if err != nil {
		log.Fatal().Err(err).Msg("could not init states")
	}

	transactionResults := storagebadger.NewTransactionResults(db)
	err = removeExecutionResultsFromHeight(protoState, execState, transactionResults, flagFromHeight)
	if err != nil {
		log.Fatal().Err(err).Msg("could not remove results")
	}

	log.Info().Msgf("all results from height %v have been removed successfully", flagFromHeight)
}

func initStates(db *badger.DB, executionStateDir string) (protocol.State, state.ExecutionState, error) {

	metrics := &metrics.NoopCollector{}
	tracer := trace.NewNoopTracer()
	distributor := events.NewDistributor()

	headers := storagebadger.NewHeaders(metrics, db)
	guarantees := storagebadger.NewGuarantees(metrics, db)
	seals := storagebadger.NewSeals(metrics, db)
	index := storagebadger.NewIndex(metrics, db)
	payloads := storagebadger.NewPayloads(db, index, guarantees, seals)
	blocks := storagebadger.NewBlocks(db, headers, payloads)
	setups := storagebadger.NewEpochSetups(metrics, db)
	commits := storagebadger.NewEpochCommits(metrics, db)
	statuses := storagebadger.NewEpochStatuses(metrics, db)

	protoState, err := protocolbadger.NewState(
		metrics,
		tracer,
		db,
		headers,
		seals,
		index,
		payloads,
		blocks,
		setups,
		commits,
		statuses,
		distributor,
	)

	if err != nil {
		return nil, nil, fmt.Errorf("could not init protocol state: %w", err)
	}

	results := storagebadger.NewExecutionResults(db)
	receipts := storagebadger.NewExecutionReceipts(db, results)
	chunkDataPacks := storagebadger.NewChunkDataPacks(db)
	stateCommitments := storagebadger.NewCommits(metrics, db)
	transactions := storagebadger.NewTransactions(metrics, db)
	collections := storagebadger.NewCollections(db, transactions)

	ledgerStorage, err := ledger.NewLedger(executionStateDir, 100, metrics, zerolog.Nop(), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("could not init ledger: %w", err)
	}

	execState := state.NewExecutionState(
		ledgerStorage,
		stateCommitments,
		blocks,
		collections,
		chunkDataPacks,
		results,
		receipts,
		db,
		tracer,
	)

	return protoState, execState, nil
}

func removeExecutionResultsFromHeight(protoState protocol.State, execState state.ExecutionState, transactionResults storage.TransactionResults, fromHeight uint64) error {
	log.Info().Msgf("removing results for blocks from height: %v", flagFromHeight)

	root, err := protoState.Params().Root()
	if err != nil {
		return fmt.Errorf("could not get root: %w", err)
	}

	if fromHeight <= root.Height {
		return fmt.Errorf("can only remove results for block above root block. fromHeight: %v, rootHeight: %v", fromHeight, root.Height)
	}

	final, err := protoState.Final().Head()
	if err != nil {
		return fmt.Errorf("could get not finalized height: %w", err)
	}

	count := 0
	total := int(final.Height-fromHeight) + 1

	// removing for finalized blocks
	for height := fromHeight; height <= final.Height; height++ {
		head, err := protoState.AtHeight(height).Head()
		if err != nil {
			return fmt.Errorf("could not get header at height: %w", err)
		}

		blockID := head.ID()

		err = removeForBlockID(execState, transactionResults, blockID)
		if err != nil {
			return fmt.Errorf("could not remove result for finalized block: %v, %w", blockID, err)
		}

		count++
		log.Info().Msgf("result at height :%v has been removed. progress (%v/%v)", height, count, total)
	}

	// removing for pending blocks
	pendings, err := protoState.Final().Pending()
	if err != nil {
		return fmt.Errorf("could not get pending block: %w", err)
	}

	count = 0
	total = len(pendings)

	for _, pending := range pendings {
		err = removeForBlockID(execState, transactionResults, pending)
		if err != nil {
			return fmt.Errorf("could not remove result for pending block: %v, %w", pending, err)
		}

		count++
		log.Info().Msgf("result for pending block :%v has been removed. progress (%v/%v) ", pending, count, total)
	}

	// highest executed is one below the fromHeight
	highest, err := protoState.AtHeight(fromHeight - 1).Head()
	if err != nil {
		return fmt.Errorf("could not get highest: %w", err)
	}

	err = execState.SetHighestExecuted(highest)
	if err != nil {
		return fmt.Errorf("failed to set highest executed")
	}

	return nil
}

func removeForBlockID(execState state.ExecutionState, transactionResults storage.TransactionResults, blockID flow.Identifier) error {
	err := execState.RemoveByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not remove by block ID: %v, %w", blockID, err)
	}

	err = transactionResults.RemoveByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not remove transaction results by BlockID: %v, %w", blockID, err)
	}

	return nil
}
