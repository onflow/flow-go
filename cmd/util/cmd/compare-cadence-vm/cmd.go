package compare_cadence_vm

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/kr/pretty"
	sdk "github.com/onflow/flow-go-sdk"
	client "github.com/onflow/flow-go-sdk/access/grpc"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	otelTrace "go.opentelemetry.io/otel/sdk/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	debug_tx "github.com/onflow/flow-go/cmd/util/cmd/debug-tx"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/utils/debug"
)

var (
	flagAccessAddress       string
	flagExecutionAddress    string
	flagChain               string
	flagComputeLimit        uint64
	flagUseExecutionDataAPI bool
	flagBlockIDs            string
	flagBlockCount          int
	flagLogTraces           bool
	flagWriteTraces         bool
	flagParallel            int
	flagSubscribe           bool
	flagSubscriptionDelay   time.Duration
	flagBatchSize           int
	flagProgressFile        string
	flagResume              bool
)

var Cmd = &cobra.Command{
	Use:   "compare-cadence-vm",
	Short: "compare execution between Cadence interpreter and Cadence VM",
	Run:   run,
}

func init() {

	Cmd.Flags().StringVar(
		&flagChain,
		"chain",
		"",
		"Chain name",
	)
	_ = Cmd.MarkFlagRequired("chain")

	Cmd.Flags().StringVar(&flagAccessAddress, "access-address", "", "address of the access node")
	_ = Cmd.MarkFlagRequired("access-address")

	Cmd.Flags().StringVar(&flagExecutionAddress, "execution-address", "", "address of the execution node (required if --use-execution-data-api is false)")

	Cmd.Flags().Uint64Var(&flagComputeLimit, "compute-limit", flow.DefaultMaxTransactionGasLimit, "transaction compute limit")

	Cmd.Flags().BoolVar(&flagUseExecutionDataAPI, "use-execution-data-api", true, "use the execution data API (default: true)")

	Cmd.Flags().StringVar(&flagBlockIDs, "block-ids", "", "block IDs, comma-separated. if --block-count > 1 is used, provide a single block ID")

	Cmd.Flags().IntVar(&flagBlockCount, "block-count", 1, "number of blocks to process (default: 1). if > 1, provide a single block ID with --block-ids")

	Cmd.Flags().BoolVar(&flagLogTraces, "log-traces", false, "log traces")

	Cmd.Flags().BoolVar(&flagWriteTraces, "write-traces", false, "write traces for mismatched transactions")

	Cmd.Flags().IntVar(&flagParallel, "parallel", 1, "number of blocks to process in parallel (default: 1)")

	Cmd.Flags().BoolVar(&flagSubscribe, "subscribe", false, "subscribe to new sealed blocks and compare them as they arrive")

	Cmd.Flags().DurationVar(&flagSubscriptionDelay, "subscription-delay", 1*time.Minute, "delay after receiving a new sealed block before comparing it")

	Cmd.Flags().IntVar(&flagBatchSize, "batch-size", 0, "number of blocks to compare per batch. the progress is recorded after each batch, so that an interrupted run can be resumed (default: 0, compare all blocks in a single batch)")

	Cmd.Flags().StringVar(&flagProgressFile, "progress-file", "", "path of the file which records the progress (default: a file named after the compared blocks, in the current directory)")

	Cmd.Flags().BoolVar(&flagResume, "resume", false, "continue the run recorded in the progress file, repeating only the batch which was interrupted. all other flags except --batch-size must be the same as in the interrupted run")
}

func run(_ *cobra.Command, args []string) {

	validateBatchingFlags()

	chainID := flow.ChainID(flagChain)
	chain := chainID.Chain()

	flowClient, err := client.NewClient(
		flagAccessAddress,
		client.WithGRPCDialOptions(
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(commonrpc.DefaultAccessMaxResponseSize)),
			//grpc.WithKeepaliveParams(keepalive.ClientParameters{
			//	Time:    10 * time.Second,
			//	Timeout: 1 * time.Hour,
			//}),
		),
	)

	var remoteClient debug.RemoteClient
	if flagUseExecutionDataAPI {
		remoteClient, err = debug.NewExecutionDataRemoteClient(flagAccessAddress, chain)
	} else if flagExecutionAddress != "" {
		remoteClient, err = debug.NewExecutionNodeRemoteClient(flagExecutionAddress)
	} else {
		log.Fatal().Msg("either --use-execution-data-api or --execution-address must be provided")
	}
	if err != nil {
		log.Fatal().Err(err).Msg("failed to remote client")
	}
	defer remoteClient.Close()

	var blockIDs []flow.Identifier
	for _, rawBlockID := range strings.Split(flagBlockIDs, ",") {
		if rawBlockID == "" {
			continue
		}

		blockID, err := flow.HexStringToIdentifier(rawBlockID)
		if err != nil {
			log.Fatal().Err(err).Str("ID", rawBlockID).Msg("failed to parse block ID")
		}

		blockIDs = append(blockIDs, blockID)
	}

	if flagSubscribe {
		if len(blockIDs) > 1 {
			log.Fatal().Msg("when using --subscribe, provide a single block ID to start from, or none to start from latest")
		}

		compareNewBlocks(blockIDs, flowClient, remoteClient, chain)
	} else {
		if len(blockIDs) == 0 {
			log.Fatal().Msg("at least one block ID must be provided")
		}
		compareBlocks(blockIDs, flowClient, remoteClient, chain)
	}
}

func compareNewBlocks(blockIDs []flow.Identifier, flowClient *client.Client, remoteClient debug.RemoteClient, chain flow.Chain) {

	var (
		blocksMismatched int64
		blocksMatched    int64
		txMismatched     int64
		txMatched        int64
	)

	g, _ := errgroup.WithContext(context.Background())
	g.SetLimit(flagParallel)

	var lastBlockID flow.Identifier

	if len(blockIDs) > 0 {
		lastBlockID = blockIDs[0]
	}

reconnect:
	for {
		const blockStatus = flow.BlockStatusSealed
		var getBlockHeader func() (*flow.Header, error)
		if lastBlockID != flow.ZeroID {
			getBlockHeader = debug_tx.SubscribeBlockHeadersFromStartBlockID(flowClient, lastBlockID, blockStatus)
		} else {
			getBlockHeader = debug_tx.SubscribeBlockHeadersFromLatest(flowClient, blockStatus)
		}

		for {
			log.Info().Msg("Waiting for new sealed block ...")
			header, err := getBlockHeader()
			if err == io.EOF {
				return
			}
			if err != nil {
				log.Warn().Err(err).Msg("failed to receive new block header")
				continue reconnect
			}

			log.Info().Msgf("New sealed block received: %s (height %d)", header.ID(), header.Height)

			lastBlockID = header.ID()

			g.Go(func() error {

				time.Sleep(flagSubscriptionDelay)

				result := compareBlock(
					header.ID(),
					header,
					remoteClient,
					flowClient,
					chain,
				)

				atomic.AddInt64(&txMismatched, int64(result.mismatches))
				atomic.AddInt64(&txMatched, int64(result.matches))
				if result.mismatches > 0 {
					atomic.AddInt64(&blocksMismatched, 1)
				} else {
					atomic.AddInt64(&blocksMatched, 1)
				}

				log.Info().Msgf("Compared %d blocks: %d matched, %d mismatched", blocksMatched+blocksMismatched, blocksMatched, blocksMismatched)
				log.Info().Msgf("Compared %d transactions: %d matched, %d mismatched", txMatched+txMismatched, txMatched, txMismatched)

				return nil
			})
		}
	}
}

// validateBatchingFlags checks the flags which control batching and resumption.
func validateBatchingFlags() {
	if flagBatchSize < 0 {
		log.Fatal().Msgf("--batch-size must not be negative, but is %d", flagBatchSize)
	}

	if flagSubscribe && (flagBatchSize != 0 || flagProgressFile != "" || flagResume) {
		log.Fatal().Msg("--batch-size, --progress-file, and --resume can not be used with --subscribe")
	}
}

type block struct {
	id     flow.Identifier
	header *flow.Header
}

// blockLoader fetches the headers of the blocks of a run, in the order in which they are compared.
//
// The blocks of a run are either the explicitly provided block IDs,
// or the block with the provided ID and its ancestors, when a block count is provided.
//
// NOT CONCURRENCY SAFE!
type blockLoader struct {
	flowClient *client.Client

	// explicitBlockIDs are the blocks to compare, if they were provided explicitly.
	// It is empty if the blocks are instead determined by following the parent of each block.
	explicitBlockIDs []flow.Identifier

	// nextBlockID is the ID of the first block which was not loaded yet.
	// It is the zero identifier once all blocks of the run were loaded.
	nextBlockID flow.Identifier

	// loadedBlockCount is the number of blocks which were loaded so far.
	loadedBlockCount int

	// totalBlockCount determines when nextBlockID is cleared after the final block is loaded.
	totalBlockCount int
}

// loadNextBlocks fetches the headers of the next count blocks.
func (l *blockLoader) loadNextBlocks(count int) []block {

	var headerProgress util.LogProgressFunc[int]
	if len(l.explicitBlockIDs) == 0 {
		headerProgress = util.LogProgress(
			log.Logger,
			util.NewLogProgressConfig(
				"fetching block headers",
				count,
				1*time.Second,
				100/5, // log every 5%
			),
		)
	}

	blocks := make([]block, 0, count)

	for i := 0; i < count; i++ {
		if l.nextBlockID == flow.ZeroID {
			log.Fatal().Msgf("no block left to compare, but %d more were requested", count-i)
		}

		blockID := l.nextBlockID
		header := debug_tx.FetchBlockHeader(blockID, l.flowClient)

		blocks = append(blocks, block{
			id:     blockID,
			header: header,
		})

		l.loadedBlockCount++
		l.advance(header)

		if headerProgress != nil {
			headerProgress(1)
		}
	}

	return blocks
}

// advance moves the loader to the next block in the run.
// It clears nextBlockID once the requested number of blocks has been loaded;
// otherwise, it selects either the loaded block's parent or the next explicit block ID.
func (l *blockLoader) advance(header *flow.Header) {
	if l.loadedBlockCount >= l.totalBlockCount {
		l.nextBlockID = flow.ZeroID
		return
	}

	if len(l.explicitBlockIDs) == 0 {
		l.nextBlockID = header.ParentID
		return
	}

	l.nextBlockID = l.explicitBlockIDs[l.loadedBlockCount]
}

func compareBlocks(
	blockIDs []flow.Identifier,
	flowClient *client.Client,
	remoteClient debug.RemoteClient,
	chain flow.Chain,
) {
	followsParents := flagBlockCount != 1

	totalBlockCount := len(blockIDs)
	if followsParents {
		if len(blockIDs) > 1 {
			log.Fatal().Msg("either provide a single block ID and use --block-count, or provide multiple block IDs and do not use --block-count")
		}

		totalBlockCount = flagBlockCount
	}

	// A run without an explicit batch size compares all blocks in a single batch.
	batchSize := flagBatchSize
	if batchSize == 0 {
		batchSize = totalBlockCount
	}

	config := runConfig{
		Chain:        flagChain,
		BlockIDs:     blockIDStrings(blockIDs),
		BlockCount:   flagBlockCount,
		ComputeLimit: flagComputeLimit,
	}

	progressFilePath := resolveProgressFilePath(blockIDs[0], totalBlockCount)
	if progressFilePath != "" {
		log.Info().Msgf("Recording the progress of this run in %s", progressFileLocation(progressFilePath))
	}

	progress := startRunProgress(progressFilePath, config, totalBlockCount, blockIDs[0])

	loader := &blockLoader{
		flowClient:       flowClient,
		loadedBlockCount: progress.CompletedBlockCount,
		totalBlockCount:  totalBlockCount,
	}
	if !followsParents {
		loader.explicitBlockIDs = blockIDs
	}

	completedBlockCount := progress.CompletedBlockCount
	stats := progress.Stats

	if completedBlockCount < totalBlockCount {
		nextBlockID, err := progress.nextBlockID()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to determine the block to continue at")
		}
		loader.nextBlockID = nextBlockID
	}

	blockProgress := util.LogProgress(
		log.Logger,
		util.NewLogProgressConfig(
			"executing blocks",
			totalBlockCount,
			1*time.Second,
			100/5, // log every 5%
		),
	)

	if completedBlockCount >= totalBlockCount {
		log.Info().Msgf("All %d blocks of this run were already compared", totalBlockCount)
	} else if completedBlockCount > 0 {
		log.Info().Msgf(
			"Continuing at block %s: %d of %d blocks were already compared",
			loader.nextBlockID,
			completedBlockCount,
			totalBlockCount,
		)

		blockProgress(completedBlockCount)
	}

	for completedBlockCount < totalBlockCount {
		batchBlockCount := min(batchSize, totalBlockCount-completedBlockCount)

		log.Info().Msgf(
			"Comparing blocks %d to %d of %d, starting at block %s ...",
			completedBlockCount+1,
			completedBlockCount+batchBlockCount,
			totalBlockCount,
			loader.nextBlockID,
		)

		blocks := loader.loadNextBlocks(batchBlockCount)

		// Report the progress within the batch, in addition to the progress of the whole run.
		// A run with a single batch would report the same progress twice.
		var batchProgress util.LogProgressFunc[int]
		if batchBlockCount < totalBlockCount {
			batchProgress = util.LogProgress(
				log.Logger,
				util.NewLogProgressConfig(
					"executing blocks in current batch",
					batchBlockCount,
					1*time.Second,
					100/5, // log every 5%
				),
			)
		}

		stats.add(compareBatch(
			blocks,
			flowClient,
			remoteClient,
			chain,
			blockProgress,
			batchProgress,
		))

		completedBlockCount += batchBlockCount

		if progressFilePath != "" {
			progress.CompletedBlockCount = completedBlockCount
			progress.NextBlockID = blockIDString(loader.nextBlockID)
			progress.Stats = stats

			if err := writeRunProgress(progressFilePath, progress); err != nil {
				// The batch was compared, but its progress was not recorded.
				// Continuing would record the following batches as if this batch had been recorded,
				// so the run stops instead, and repeats this batch when it is resumed.
				log.Fatal().Err(err).Msgf(
					"failed to record the progress after comparing %d blocks",
					completedBlockCount,
				)
			}
		}

		// Report the results so far, so that a long run which is interrupted
		// still reported the results of all batches it completed.
		if completedBlockCount < totalBlockCount {
			logStats(stats)
		}
	}

	logStats(stats)
}

// compareBatch compares the given blocks and returns their combined results.
//
// Each compared block is reported to the progress of the whole run,
// and to the progress within the batch, if there is more than one batch.
func compareBatch(
	blocks []block,
	flowClient *client.Client,
	remoteClient debug.RemoteClient,
	chain flow.Chain,
	overallProgress util.LogProgressFunc[int],
	batchProgress util.LogProgressFunc[int],
) runStats {

	var stats runStats

	g, _ := errgroup.WithContext(context.Background())
	g.SetLimit(flagParallel)

	for _, block := range blocks {

		g.Go(func() error {
			result := compareBlock(
				block.id,
				block.header,
				remoteClient,
				flowClient,
				chain,
			)

			atomic.AddInt64(&stats.TransactionsMismatched, int64(result.mismatches))
			atomic.AddInt64(&stats.TransactionsMatched, int64(result.matches))
			if result.mismatches > 0 {
				atomic.AddInt64(&stats.BlocksMismatched, 1)
			} else {
				atomic.AddInt64(&stats.BlocksMatched, 1)
			}

			overallProgress(1)
			if batchProgress != nil {
				batchProgress(1)
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		log.Fatal().Err(err).Msg("failed to compare blocks")
	}

	return stats
}

// resolveProgressFilePath returns the path of the file which records the progress of the run,
// or the empty string if the run does not record its progress.
//
// A run which compares all blocks in a single batch has no intermediate progress to record,
// so it only records its progress if a progress file was requested explicitly.
func resolveProgressFilePath(firstBlockID flow.Identifier, totalBlockCount int) string {
	if flagProgressFile != "" {
		return flagProgressFile
	}

	if flagBatchSize == 0 && !flagResume {
		return ""
	}

	return defaultProgressFileName(firstBlockID, totalBlockCount)
}

// defaultProgressFileName returns the name of the progress file for a run
// which did not request a particular one.
//
// The name is derived from the compared blocks, so that resuming the same run
// finds the progress file of that run in the directory the command is run in.
func defaultProgressFileName(firstBlockID flow.Identifier, totalBlockCount int) string {
	return fmt.Sprintf(
		"compare-cadence-vm-%s-%d.progress.json",
		firstBlockID.String()[:16],
		totalBlockCount,
	)
}

// progressFileLocation returns the absolute path of the progress file, for reporting it to the user.
// It falls back to the given path if the absolute path can not be determined.
func progressFileLocation(path string) string {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return absolutePath
}

// startRunProgress returns the progress which the run continues from.
//
// A run which is not resumed refuses to discard the progress of a previous run,
// and records that it has not compared any block yet, so that a progress file which can not be
// written is reported before any block is compared, instead of after the first batch.
func startRunProgress(
	progressFilePath string,
	config runConfig,
	totalBlockCount int,
	firstBlockID flow.Identifier,
) runProgress {

	if flagResume {
		progress, err := readRunProgress(progressFilePath)
		if err != nil {
			log.Fatal().Err(err).Msgf(
				"failed to read the progress file %s. run without --resume to start a new run",
				progressFileLocation(progressFilePath),
			)
		}

		if err := progress.validate(config, totalBlockCount); err != nil {
			log.Fatal().Err(err).Msg("failed to resume the recorded run")
		}

		return progress
	}

	progress := newRunProgress(config, firstBlockID)

	if progressFilePath == "" {
		return progress
	}

	exists, err := runProgressExists(progressFilePath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to check for an existing progress file")
	}
	if exists {
		log.Fatal().Msgf(
			"progress file %s already exists. use --resume to continue the recorded run, or remove the file to start over",
			progressFileLocation(progressFilePath),
		)
	}

	if err := writeRunProgress(progressFilePath, progress); err != nil {
		log.Fatal().Err(err).Msgf(
			"failed to create the progress file %s",
			progressFileLocation(progressFilePath),
		)
	}

	return progress
}

// blockIDStrings returns the hexadecimal representations of the given block IDs.
func blockIDStrings(blockIDs []flow.Identifier) []string {
	strs := make([]string, 0, len(blockIDs))
	for _, blockID := range blockIDs {
		strs = append(strs, blockID.String())
	}
	return strs
}

func logStats(stats runStats) {
	log.Info().Msgf(
		"Compared %d blocks: %d matched, %d mismatched",
		stats.BlocksMatched+stats.BlocksMismatched,
		stats.BlocksMatched,
		stats.BlocksMismatched,
	)
	log.Info().Msgf(
		"Compared %d transactions: %d matched, %d mismatched",
		stats.TransactionsMatched+stats.TransactionsMismatched,
		stats.TransactionsMatched,
		stats.TransactionsMismatched,
	)
}

type blockResult struct {
	mismatches int
	matches    int
}

func compareBlock(
	blockID flow.Identifier,
	header *flow.Header,
	remoteClient debug.RemoteClient,
	flowClient *client.Client,
	chain flow.Chain,
) (
	result blockResult,
) {

	var (
		blockTransactions []*sdk.Transaction
		systemTxID        sdk.Identifier
	)
	blockTransactions, systemTxID = debug_tx.FetchBlockTransactions(blockID, flowClient)

	log.Info().Msgf("Running all transactions in block %s (height %d) ...", blockID, header.Height)

	log.Info().Msg("Running with interpreter ...")

	interBlockSnapshot := debug_tx.NewBlockSnapshot(remoteClient, header)

	fvmOptions := []fvm.Option{
		fvm.WithEntropyProvider(debug_tx.BlockHashEntropyProvider{
			BlockHash: blockID,
		}),
	}

	var (
		interSpanExporters []*debug.InterestingCadenceSpanExporter
		interTxSnapshots   []*debug.CapturingStorageSnapshot
	)
	interResults := debug_tx.RunBlock(
		interBlockSnapshot,
		header,
		blockTransactions,
		flow.ZeroID,
		systemTxID,
		chain,
		false,
		func(_ flow.Identifier, snapshot debug.UpdatableStorageSnapshot) debug.UpdatableStorageSnapshot {
			txSnapshot := debug.NewCapturingStorageSnapshot(snapshot)
			interTxSnapshots = append(interTxSnapshots, txSnapshot)
			return txSnapshot
		},
		func(_ flow.Identifier) otelTrace.SpanExporter {
			exporter := &debug.InterestingCadenceSpanExporter{
				Log: flagLogTraces,
			}
			interSpanExporters = append(interSpanExporters, exporter)
			return exporter
		},
		flagComputeLimit,
		fvmOptions,
	)

	log.Info().Msg("Running with VM ...")

	vmBlockSnapshot := debug_tx.NewBlockSnapshot(remoteClient, header)

	var (
		vmSpanExporters []*debug.InterestingCadenceSpanExporter
		vmTxSnapshots   []*debug.CapturingStorageSnapshot
	)
	vmResults := debug_tx.RunBlock(
		vmBlockSnapshot,
		header,
		blockTransactions,
		flow.ZeroID,
		systemTxID,
		chain,
		true,
		func(_ flow.Identifier, snapshot debug.UpdatableStorageSnapshot) debug.UpdatableStorageSnapshot {
			txSnapshot := debug.NewCapturingStorageSnapshot(snapshot)
			vmTxSnapshots = append(vmTxSnapshots, txSnapshot)
			return txSnapshot
		},
		func(_ flow.Identifier) otelTrace.SpanExporter {
			exporter := &debug.InterestingCadenceSpanExporter{
				Log: flagLogTraces,
			}
			vmSpanExporters = append(vmSpanExporters, exporter)
			return exporter
		},
		flagComputeLimit,
		fvmOptions,
	)

	var mismatch bool

	for i, interResult := range interResults {
		vmResult := vmResults[i]
		transaction := blockTransactions[i]

		txID := flow.Identifier(transaction.ID())

		if !compareResults(
			txID,
			interResult,
			vmResult,
		) {
			mismatch = true

			result.mismatches++

			if flagWriteTraces {
				writeTraces(txID, "inter", interSpanExporters[i])
				writeTraces(txID, "vm", vmSpanExporters[i])
			}
		} else {
			result.matches++
		}
	}

	if mismatch {
		log.Error().Msgf("Block %s (height %d) did not match!", blockID, header.Height)
	} else {
		log.Info().Msgf("Block %s (height %d) matched!", blockID, header.Height)
	}

	return result
}

func writeTraces(id flow.Identifier, kind string, exporter *debug.InterestingCadenceSpanExporter) {

	f, err := os.Create(fmt.Sprintf("%s.%s.txt", id, kind))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create trace file")
	}
	defer f.Close()

	err = exporter.WriteSpans(f)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to write interpreter spans")
	}
}

func compareResults(txID flow.Identifier, interResult debug.Result, vmResult debug.Result) bool {
	log := log.With().Str("tx", txID.String()).Logger()

	var mismatch bool

	// Compare errors (just presence/absence of error, not the error message itself)

	interErr := interResult.Output.Err
	vmErr := vmResult.Output.Err

	if interErr == nil && vmErr != nil {

		if vmErr.Code() == errors.ErrCodeComputationLimitExceededError {
			log.Warn().Msg("VM exceeded computation limit but interpreter succeeded. Ignoring")
			return true
		}

		log.Error().Msg("VM failed but interpreter succeeded")
		mismatch = true

	} else if interErr != nil && vmErr == nil {
		if interErr.Code() == errors.ErrCodeComputationLimitExceededError {
			log.Warn().Msg("Interpreter exceeded computation limit but VM succeeded. Ignoring")
			return true
		}

		log.Error().Msg("Interpreter failed but VM succeeded")
		mismatch = true
	} else if interErr != nil &&
		vmErr != nil &&
		interErr.Code() == errors.ErrCodeComputationLimitExceededError &&
		vmErr.Code() == errors.ErrCodeComputationLimitExceededError {

		log.Warn().Msg("Both interpreter and VM exceeded computation limit. Ignoring")
		return true
	}

	// Compare events

	interEventCount := len(interResult.Output.Events)
	vmEventCount := len(vmResult.Output.Events)
	if interEventCount != vmEventCount {
		log.Error().Msgf("Number of events differ: interpreter %d vs VM %d", interEventCount, vmEventCount)
		mismatch = true
	}

	eventsDiffs := pretty.Diff(interResult.Output.Events, vmResult.Output.Events)
	if len(eventsDiffs) != 0 {
		mismatch = true
	}
	for _, diff := range eventsDiffs {
		log.Error().Msgf("Event diff: %s", diff)
	}

	// Compare logs

	interLogCount := len(interResult.Output.Logs)
	vmLogCount := len(vmResult.Output.Logs)
	if interLogCount != vmLogCount {
		log.Error().Msgf(
			"Number of logs differ: interpreter %d vs VM %d",
			interLogCount,
			vmLogCount,
		)
		mismatch = true
	}

	logsDiffs := pretty.Diff(interResult.Output.Logs, vmResult.Output.Logs)
	if len(logsDiffs) != 0 {
		mismatch = true
	}
	for _, diff := range logsDiffs {
		log.Error().Msgf("Log diff: %s", diff)
	}

	// Compare set of read register IDs.
	// The VM might perform fewer or more reads than the interpreter,
	// and still produce the same end result.
	// This is not considered a mismatch, but we warn about it.

	interReadRegisterIDs := interResult.Snapshot.ReadRegisterSet()

	vmReadRegisterIDs := vmResult.Snapshot.ReadRegisterSet()

	if len(vmReadRegisterIDs) != len(interReadRegisterIDs) {
		log.Warn().Msgf(
			"Number of read registers differ: interpreter %d vs VM %d",
			len(interReadRegisterIDs),
			len(vmReadRegisterIDs),
		)
	}

	var vmMissingReadRegisterIDs []flow.RegisterID
	for id := range interReadRegisterIDs {
		if _, ok := vmReadRegisterIDs[id]; !ok {
			vmMissingReadRegisterIDs = append(vmMissingReadRegisterIDs, id)
		}
	}
	debug.SortRegisterIDs(vmMissingReadRegisterIDs)

	if len(vmMissingReadRegisterIDs) > 0 {
		log.Warn().Msgf("Interpreter read registers but VM did not: %s", vmMissingReadRegisterIDs)
	}

	var interMissingReadRegisterIDs []flow.RegisterID
	for id := range vmReadRegisterIDs {
		if _, ok := interReadRegisterIDs[id]; !ok {
			interMissingReadRegisterIDs = append(interMissingReadRegisterIDs, id)
		}
	}
	debug.SortRegisterIDs(interMissingReadRegisterIDs)

	if len(interMissingReadRegisterIDs) > 0 {
		log.Warn().Msgf("VM read registers but interpreter did not: %s", interMissingReadRegisterIDs)
	}

	// Compare set of written register entries (IDs and values).

	interWrittenRegisterEntries := interResult.Snapshot.UpdatedRegisters()
	debug.SortRegisterEntries(interWrittenRegisterEntries)

	vmWrittenRegisterEntries := vmResult.Snapshot.UpdatedRegisters()
	debug.SortRegisterEntries(vmWrittenRegisterEntries)

	if len(vmWrittenRegisterEntries) != len(interWrittenRegisterEntries) {
		log.Error().Msgf(
			"Number of written registers differ: interpreter %d vs VM %d",
			len(interWrittenRegisterEntries),
			len(vmWrittenRegisterEntries),
		)
		mismatch = true
	}

	for i, interWrittenRegisterEntry := range interWrittenRegisterEntries {
		if i >= len(vmWrittenRegisterEntries) {
			break
		}
		vmWrittenRegisterEntry := vmWrittenRegisterEntries[i]

		if interWrittenRegisterEntry.Key != vmWrittenRegisterEntry.Key {
			log.Error().Msgf(
				"Written register ID mismatch at index %d: interpreter %s vs VM %s",
				i,
				interWrittenRegisterEntry.Key,
				vmWrittenRegisterEntry.Key,
			)
			mismatch = true

		} else if !bytes.Equal(interWrittenRegisterEntry.Value, vmWrittenRegisterEntry.Value) {
			log.Error().Msgf(
				"Written register value mismatch for register %s: interpreter %q vs VM %q",
				interWrittenRegisterEntry.Key,
				hex.EncodeToString(interWrittenRegisterEntry.Value),
				hex.EncodeToString(vmWrittenRegisterEntry.Value),
			)
			mismatch = true
		}
	}

	// Compare SPOCKs.
	// The VM might perform fewer or more reads, or reads in a different order than the interpreter,
	// and still produce the same end result.
	// This is not considered a mismatch, but we warn about it.

	interSpock := interResult.Snapshot.SpockSecret
	vmSpock := vmResult.Snapshot.SpockSecret

	if !bytes.Equal(interSpock, vmSpock) {
		log.Warn().Msgf(
			"SPOCKs differ: interpreter %x vs VM %x",
			interSpock,
			vmSpock,
		)
	}

	if mismatch {
		log.Error().Msg("Differences found between interpreter and VM")
	} else {
		log.Info().Msg("No differences found between interpreter and VM")
	}

	return !mismatch
}
