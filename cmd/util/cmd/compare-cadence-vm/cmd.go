package compare_cadence_vm

import (
	"bytes"
	"context"
	"encoding/hex"
	"os"
	"sync/atomic"

	"github.com/kr/pretty"
	sdk "github.com/onflow/flow-go-sdk"
	client "github.com/onflow/flow-go-sdk/access/grpc"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	otelTrace "go.opentelemetry.io/otel/sdk/trace"
	"golang.org/x/exp/slices"
	"golang.org/x/sync/errgroup"

	debug_tx "github.com/onflow/flow-go/cmd/util/cmd/debug-tx"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/grpcclient"
	"github.com/onflow/flow-go/utils/debug"
)

var (
	flagAccessAddress       string
	flagExecutionAddress    string
	flagChain               string
	flagComputeLimit        uint64
	flagUseExecutionDataAPI bool
	flagBlockID             string
	flagBlockCount          int
	flagDebug               bool
	flagParallel            int
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

	Cmd.Flags().Uint64Var(&flagComputeLimit, "compute-limit", 9999, "transaction compute limit")

	Cmd.Flags().BoolVar(&flagUseExecutionDataAPI, "use-execution-data-api", true, "use the execution data API (default: true)")

	Cmd.Flags().StringVar(&flagBlockID, "block-id", "", "block ID")
	_ = Cmd.MarkFlagRequired("block-id")

	Cmd.Flags().IntVar(&flagBlockCount, "block-count", 1, "number of blocks to process (default: 1)")

	Cmd.Flags().BoolVar(&flagDebug, "debug", false, "enable debug logging")

	Cmd.Flags().IntVar(&flagParallel, "parallel", 1, "number of blocks to process in parallel (default: 1)")
}

func run(_ *cobra.Command, args []string) {

	chainID := flow.ChainID(flagChain)
	chain := chainID.Chain()

	config, err := grpcclient.NewFlowClientConfig(flagAccessAddress, "", flow.ZeroID, true)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create flow client config")
	}

	flowClient, err := grpcclient.FlowClient(config)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create client")
	}

	var remoteClient debug.RemoteClient
	if flagUseExecutionDataAPI {
		remoteClient, err = debug.NewExecutionDataRemoteClient(flagAccessAddress)
	} else if flagExecutionAddress != "" {
		remoteClient, err = debug.NewExecutionNodeRemoteClient(flagExecutionAddress)
	} else {
		log.Fatal().Msg("either --use-execution-data-api or --execution-address must be provided")
	}
	if err != nil {
		log.Fatal().Err(err).Msg("failed to remote client")
	}
	defer remoteClient.Close()

	blockID, err := flow.HexStringToIdentifier(flagBlockID)
	if err != nil {
		log.Fatal().Err(err).Str("ID", flagBlockID).Msg("failed to parse block ID")
	}

	type block struct {
		id     flow.Identifier
		header *flow.Header
	}

	var blocks []block

	for i := 0; i < flagBlockCount; i++ {
		header := debug_tx.FetchBlockHeader(blockID, flowClient)

		blocks = append(blocks, block{
			id:     blockID,
			header: header,
		})

		blockID = header.ParentID
	}

	var (
		blocksMismatched int64
		blocksMatched    int64
		txMismatched     int64
		txMatched        int64
	)

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

			atomic.AddInt64(&txMismatched, int64(result.mismatches))
			atomic.AddInt64(&txMatched, int64(result.matches))
			if result.mismatches > 0 {
				atomic.AddInt64(&blocksMismatched, 1)
			} else {
				atomic.AddInt64(&blocksMatched, 1)
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		log.Fatal().Err(err).Msg("failed to compare blocks")
	}

	log.Info().Msgf("Compared %d blocks: %d matched, %d mismatched", blocksMatched+blocksMismatched, blocksMatched, blocksMismatched)
	log.Info().Msgf("Compared %d transactions: %d matched, %d mismatched", txMatched+txMismatched, txMatched, txMismatched)
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
			exporter := &debug.InterestingCadenceSpanExporter{}
			interSpanExporters = append(interSpanExporters, exporter)
			return exporter
		},
		flagComputeLimit,
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
			exporter := &debug.InterestingCadenceSpanExporter{}
			vmSpanExporters = append(vmSpanExporters, exporter)
			return exporter
		},
		flagComputeLimit,
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

			if flagDebug {
				compareReadsAndWrites(
					txID,
					interTxSnapshots[i],
					vmTxSnapshots[i],
				)

				log.Info().Str("tx", txID.String()).Msg("Interpreter spans:")

				interSpanExporter := interSpanExporters[i]
				err := interSpanExporter.WriteSpans(os.Stdout)
				if err != nil {
					log.Fatal().Err(err).Msg("failed to write interpreter spans")
				}

				log.Info().Str("tx", txID.String()).Msg("VM spans:")

				vmSpanExporter := vmSpanExporters[i]
				err = vmSpanExporter.WriteSpans(os.Stdout)
				if err != nil {
					log.Fatal().Err(err).Msg("failed to write VM spans")
				}
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

func compareResults(txID flow.Identifier, interResult debug.Result, vmResult debug.Result) bool {
	log := log.With().Str("tx", txID.String()).Logger()

	var mismatch bool

	// Compare errors (just presence/absence of error, not the error message itself)

	interErr := interResult.Output.Err
	vmErr := vmResult.Output.Err

	if interErr == nil && vmErr != nil {
		log.Error().Msgf("VM failed but interpreter succeeded")
		mismatch = true
	} else if interErr != nil && vmErr == nil {
		log.Error().Msgf("Interpreter failed but VM succeeded")
		mismatch = true
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
				"Written register value mismatch for register %s: interpreter %s vs VM %s",
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

func compareReadsAndWrites(
	txID flow.Identifier,
	interSnapshot *debug.CapturingStorageSnapshot,
	vmSnapshot *debug.CapturingStorageSnapshot,
) {
	log := log.With().Str("tx", txID.String()).Logger()

	// Compare reads

	if len(interSnapshot.Reads) != len(vmSnapshot.Reads) {
		log.Error().Msgf(
			"Number of read registers differ: interpreter %d vs VM %d",
			len(interSnapshot.Reads),
			len(vmSnapshot.Reads),
		)
	}

	for index, interRead := range interSnapshot.Reads {
		if index >= len(vmSnapshot.Reads) {
			break
		}
		vmRead := vmSnapshot.Reads[index]

		if interRead.RegisterID != vmRead.RegisterID {
			log.Error().Msgf(
				"Read register ID mismatch at index %d: interpreter %s vs VM %s",
				index,
				interRead.RegisterID,
				vmRead.RegisterID,
			)
			continue
		}

		if !slices.Equal(interRead.RegisterValue, vmRead.RegisterValue) {
			log.Error().Msgf(
				"Read register value mismatch for register %s: interpreter %s vs VM %s",
				interRead.RegisterID,
				hex.EncodeToString(interRead.RegisterValue),
				hex.EncodeToString(vmRead.RegisterValue),
			)
		}
	}

	// Compare writes

	if len(interSnapshot.Writes) != len(vmSnapshot.Writes) {
		log.Error().Msgf(
			"Number of written registers differ: interpreter %d vs VM %d",
			len(interSnapshot.Writes),
			len(vmSnapshot.Writes),
		)
	}

	for index, interWrite := range interSnapshot.Writes {
		if index >= len(vmSnapshot.Writes) {
			break
		}
		vmWrite := vmSnapshot.Writes[index]

		if interWrite.RegisterID != vmWrite.RegisterID {
			log.Error().Msgf(
				"Written register ID mismatch at index %d: interpreter %s vs VM %s",
				index,
				interWrite.RegisterID,
				vmWrite.RegisterID,
			)
			continue
		}

		if !slices.Equal(interWrite.RegisterValue, vmWrite.RegisterValue) {
			log.Error().Msgf(
				"Written register value mismatch for register %s: interpreter %s vs VM %s",
				interWrite.RegisterID,
				hex.EncodeToString(interWrite.RegisterValue),
				hex.EncodeToString(vmWrite.RegisterValue),
			)
		}
	}
}
