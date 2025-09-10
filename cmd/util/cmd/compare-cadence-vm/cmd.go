package compare_cadence_vm

import (
	"context"
	"encoding/hex"
	"strings"

	"github.com/kr/pretty"
	client "github.com/onflow/flow-go-sdk/access/grpc"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	otelTrace "go.opentelemetry.io/otel/sdk/trace"
	"golang.org/x/exp/slices"

	debug_tx "github.com/onflow/flow-go/cmd/util/cmd/debug-tx"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/grpcclient"
	"github.com/onflow/flow-go/module/trace"
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

	var header *flow.Header

	for i := 0; i < flagBlockCount; i++ {
		if header != nil {
			blockID = header.ParentID
		}

		header = compareBlock(
			blockID,
			remoteClient,
			flowClient,
			chain,
		)
	}
}

type spans struct {
	spans []otelTrace.ReadOnlySpan
}

var _ otelTrace.SpanExporter = &spans{}

var interestingSpanNamePrefixes = []trace.SpanName{
	trace.FVMCadenceTrace,
	trace.FVMEnvAllocateSlabIndex,
}

func (s *spans) ExportSpans(_ context.Context, spans []otelTrace.ReadOnlySpan) error {
	for _, span := range spans {
		name := span.Name()
		for _, prefix := range interestingSpanNamePrefixes {
			if strings.HasPrefix(name, string(prefix)) {
				s.spans = append(s.spans, span)
				break
			}
		}
	}
	return nil
}

func (s *spans) Shutdown(_ context.Context) error {
	return nil
}

func compareBlock(
	blockID flow.Identifier,
	remoteClient debug.RemoteClient,
	flowClient *client.Client,
	chain flow.Chain,
) *flow.Header {

	blockTransactions, systemTxID, header := debug_tx.FetchBlockInfo(blockID, flowClient)

	log.Info().Msgf("Running all transactions in block %s (height %d) ...", blockID, header.Height)

	log.Info().Msg("Running with interpreter ...")

	interBlockSnapshot := debug_tx.NewBlockSnapshot(remoteClient, header)

	var (
		interSpans       []*spans
		interTxSnapshots []*debug.CapturingStorageSnapshot
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
			spans := &spans{}
			interSpans = append(interSpans, spans)
			return spans
		},
		flagComputeLimit,
	)

	log.Info().Msg("Running with VM ...")

	vmBlockSnapshot := debug_tx.NewBlockSnapshot(remoteClient, header)

	var (
		vmSpans       []*spans
		vmTxSnapshots []*debug.CapturingStorageSnapshot
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
			spans := &spans{}
			vmSpans = append(vmSpans, spans)
			return spans
		},
		flagComputeLimit,
	)

	for i, interResult := range interResults {
		vmResult := vmResults[i]
		transaction := blockTransactions[i]

		txID := flow.Identifier(transaction.ID())

		compareResults(
			txID,
			interResult,
			vmResult,
		)

		// TODO: not yet equal
		//compareSpans(
		//	txID,
		//	interSpans[i].spans,
		//	vmSpans[i].spans,
		//)

		compareReadsAndWrites(
			txID,
			interTxSnapshots[i],
			vmTxSnapshots[i],
		)
	}

	return header
}

func compareResults(
	txID flow.Identifier,
	interResult debug.Result,
	vmResult debug.Result,
) {
	log := log.With().Str("tx", txID.String()).Logger()

	var mismatch bool

	// Compare errors

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

	// Compare read registers
	interReadRegisters := interResult.Snapshot.ReadRegisterIDs()
	debug.SortRegisterIDs(interReadRegisters)

	vmReadRegisters := vmResult.Snapshot.ReadRegisterIDs()
	debug.SortRegisterIDs(vmReadRegisters)

	readRegisterDiff := pretty.Diff(interReadRegisters, vmReadRegisters)
	if len(readRegisterDiff) != 0 {
		mismatch = true
	}
	for _, diff := range readRegisterDiff {
		log.Error().Msgf("Read register diff (interpreter vs VM): %s", diff)
	}

	// Compare updated registers
	interUpdatedRegisters := interResult.Snapshot.UpdatedRegisters()
	debug.SortRegisterEntries(interUpdatedRegisters)

	vmUpdatedRegisters := vmResult.Snapshot.UpdatedRegisters()
	debug.SortRegisterEntries(vmUpdatedRegisters)

	if len(interUpdatedRegisters) != len(vmUpdatedRegisters) {
		log.Error().Msgf(
			"Number of updated registers differ: interpreter %d vs VM %d",
			len(interUpdatedRegisters),
			len(vmUpdatedRegisters),
		)
		mismatch = true
	}

	for i, interEntry := range interUpdatedRegisters {
		if i >= len(vmUpdatedRegisters) {
			break
		}
		vmEntry := vmUpdatedRegisters[i]

		if interEntry.Key != vmEntry.Key {
			log.Error().Msgf(
				"Updated register key mismatch at index %d: interpreter %s vs VM %s",
				i,
				interEntry.Key,
				vmEntry.Key,
			)
			mismatch = true
			continue
		}

		if !slices.Equal(interEntry.Value, vmEntry.Value) {
			log.Error().Msgf(
				"Updated register value mismatch for register %s: interpreter %s vs VM %s",
				interEntry.Key,
				hex.EncodeToString(interEntry.Value),
				hex.EncodeToString(vmEntry.Value),
			)
			mismatch = true
		}
	}

	if mismatch {
		log.Error().Msg("Differences found between interpreter and VM")
	} else {
		log.Info().Msg("No differences found between interpreter and VM")
	}
}

func compareSpans(
	txID flow.Identifier,
	interSpans []otelTrace.ReadOnlySpan,
	vmSpans []otelTrace.ReadOnlySpan,
) {
	log := log.With().Str("tx", txID.String()).Logger()

	diffs := pretty.Diff(interSpans, vmSpans)
	for _, diff := range diffs {
		log.Error().Msgf("Span diff: %s", diff)
	}
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
