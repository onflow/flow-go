package debug_tx

import (
	"context"
	"os"

	sdk "github.com/onflow/flow-go-sdk"
	client "github.com/onflow/flow-go-sdk/access/grpc"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	otelTrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/grpcclient"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/debug"
)

// use the following command to forward port 9000 from the EN to localhost:9001
// `gcloud compute ssh '--ssh-flag=-A' --no-user-output-enabled --tunnel-through-iap migrationmainnet1-execution-001 --project flow-multi-region -- -NL 9001:localhost:9000`

var (
	flagAccessAddress       string
	flagExecutionAddress    string
	flagChain               string
	flagComputeLimit        uint64
	flagUseExecutionDataAPI bool
	flagShowResult          bool
	flagBlockID             string
	flagTracePath           string
	flagLogCadenceTraces    bool
	flagOnlyTraceCadence    bool
	flagEntropyProvider     string
)

var Cmd = &cobra.Command{
	Use:   "debug-tx",
	Short: "debug a transaction",
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

	Cmd.Flags().BoolVar(&flagShowResult, "show-result", false, "show result (default: false)")

	Cmd.Flags().StringVar(&flagBlockID, "block-id", "", "block ID")

	Cmd.Flags().StringVar(&flagTracePath, "trace", "", "enable tracing to given path")

	Cmd.Flags().BoolVar(&flagLogCadenceTraces, "log-cadence-traces", false, "log Cadence traces. requires --trace and --only-trace-cadence to be set (default: false)")

	Cmd.Flags().BoolVar(&flagOnlyTraceCadence, "only-trace-cadence", false, "when tracing, only include spans related to Cadence execution (default: false)")

	Cmd.Flags().StringVar(&flagEntropyProvider, "entropy-provider", "none", "entropy provider to use (default: none; options: none, block-hash)")
}

func run(cmd *cobra.Command, args []string) {
	ctx := cmd.Context()

	chainID := flow.ChainID(flagChain)
	chain := chainID.Chain()

	config, err := grpcclient.NewFlowClientConfig(flagAccessAddress, "", flow.ZeroID, true)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create flow client config")
	}

	flowClient, err := grpcclient.FlowClient(config)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create client")
	}

	var remoteClient debug.RemoteClient
	if flagUseExecutionDataAPI {
		remoteClient, err = debug.NewExecutionDataRemoteClient(flagAccessAddress, chain)
	} else if flagExecutionAddress != "" {
		remoteClient, err = debug.NewExecutionNodeRemoteClient(flagExecutionAddress)
	} else {
		log.Fatal().Msg("Either --use-execution-data-api or --execution-address must be provided")
	}
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create remote client")
	}
	defer remoteClient.Close()

	var traceFile *os.File
	if flagTracePath == "-" {
		traceFile = os.Stdout
	} else if flagTracePath != "" {
		traceFile, err = os.Create(flagTracePath)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create trace file")
		}
		defer traceFile.Close()
	}

	if flagLogCadenceTraces && (flagTracePath == "" || !flagOnlyTraceCadence) {
		log.Fatal().Msg("--log-cadence-traces requires both --trace and --only-trace-cadence")
	}

	var spanExporter otelTrace.SpanExporter
	if traceFile != nil {
		if flagOnlyTraceCadence {
			cadenceSpanExporter := &debug.InterestingCadenceSpanExporter{
				Log: flagLogCadenceTraces,
			}
			defer func() {
				err = cadenceSpanExporter.WriteSpans(traceFile)
				if err != nil {
					log.Fatal().Err(err).Msg("Failed to write spans")
				}
			}()
			spanExporter = cadenceSpanExporter
		} else {
			spanExporter, err = stdouttrace.New(
				stdouttrace.WithWriter(traceFile),
				stdouttrace.WithoutTimestamps(),
			)
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to create trace exporter")
			}
		}
	}

	if flagBlockID == "" && len(args) == 0 {
		log.Fatal().Msg("Must provide either --block-id or one or more transaction IDs")
	}

	if flagBlockID != "" {

		if len(args) != 0 {
			log.Fatal().Msg("Cannot provide both block ID and transaction IDs")
		}

		// Block ID provided, fetch the block and its transaction IDs

		blockID, err := flow.HexStringToIdentifier(flagBlockID)
		if err != nil {
			log.Fatal().Err(err).Str("ID", flagBlockID).Msg("Failed to parse block ID")
		}

		header := FetchBlockHeader(ctx, blockID, flowClient)
		blockTransactions := FetchBlockTransactions(ctx, blockID, flowClient)

		log.Info().Msgf("Running all transactions in block %s (height %d) ...", blockID, header.Height)

		var newSpanExporter func(flow.Identifier) otelTrace.SpanExporter
		if spanExporter != nil {
			newSpanExporter = func(_ flow.Identifier) otelTrace.SpanExporter {
				return spanExporter
			}
		}

		blockSnapshot := NewBlockSnapshot(remoteClient, header)

		results := RunBlock(
			blockSnapshot,
			header,
			blockTransactions,
			flow.ZeroID,
			chain,
			nil,
			newSpanExporter,
			flagComputeLimit,
			fvmOptions(blockID),
		)

		if flagShowResult {
			for i, blockTx := range blockTransactions {
				debug.WriteResult(
					os.Stdout,
					flow.Identifier(blockTx.ID()),
					results[i],
				)
			}
		}

	} else {
		// No block ID provided, proceed with transaction IDs from args

		for _, rawTxID := range args {
			txID, err := flow.HexStringToIdentifier(rawTxID)
			if err != nil {
				log.Fatal().Err(err).Str("ID", rawTxID).Msg("Failed to parse transaction ID")
			}

			result := RunSingleTransaction(
				ctx,
				remoteClient,
				txID,
				flowClient,
				chain,
				spanExporter,
				flagComputeLimit,
			)
			if flagShowResult {
				debug.WriteResult(
					os.Stdout,
					txID,
					result,
				)
			}
		}
	}
}

func fvmOptions(blockID flow.Identifier) []fvm.Option {
	var options []fvm.Option

	switch flagEntropyProvider {
	case "none":
		// no entropy provider
	case "block-hash":
		options = append(
			options,
			fvm.WithEntropyProvider(BlockHashEntropyProvider{
				BlockHash: blockID,
			}),
		)
	default:
		log.Fatal().
			Str("entropy-provider", flagEntropyProvider).
			Msg("Invalid --entropy-provider value, must be one of: none, block-hash")
	}

	return options
}

func RunSingleTransaction(
	ctx context.Context,
	remoteClient debug.RemoteClient,
	txID flow.Identifier,
	flowClient *client.Client,
	chain flow.Chain,
	spanExporter otelTrace.SpanExporter,
	computeLimit uint64,
) debug.Result {
	log.Info().Msgf("Fetching transaction result for %s ...", txID)

	txResult, err := flowClient.GetTransactionResult(ctx, sdk.Identifier(txID))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to fetch transaction result")
	}

	blockID := flow.Identifier(txResult.BlockID)
	blockHeight := txResult.BlockHeight

	log.Info().Msgf(
		"Fetched transaction result: %s at block %s (height %d)",
		txResult.Status,
		blockID,
		blockHeight,
	)

	// Fetch block info
	header := FetchBlockHeader(ctx, blockID, flowClient)
	blockTransactions := FetchBlockTransactions(ctx, blockID, flowClient)

	var newSpanExporter func(blockTxID flow.Identifier) otelTrace.SpanExporter
	if spanExporter != nil {
		newSpanExporter = func(blockTxID flow.Identifier) otelTrace.SpanExporter {
			if blockTxID == txID {
				return spanExporter
			}
			return nil
		}
	}

	blockSnapshot := NewBlockSnapshot(remoteClient, header)

	results := RunBlock(
		blockSnapshot,
		header,
		blockTransactions,
		txID,
		chain,
		nil,
		newSpanExporter,
		computeLimit,
		fvmOptions(blockID),
	)

	for i, blockTx := range blockTransactions {
		if flow.Identifier(blockTx.ID()) == txID {
			return results[i]
		}
	}

	log.Fatal().Msg("Transaction not found in block transactions")

	return debug.Result{}
}

func NewBlockSnapshot(
	remoteClient debug.RemoteClient,
	blockHeader *flow.Header,
) *debug.CachingStorageSnapshot {

	remoteSnapshot, err := remoteClient.StorageSnapshot(blockHeader.Height, blockHeader.ID())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create storage snapshot")
	}

	return debug.NewCachingStorageSnapshot(remoteSnapshot)
}

func FetchBlockHeader(
	ctx context.Context,
	blockID flow.Identifier,
	flowClient *client.Client,
) (header *flow.Header) {
	log.Info().Msg("Fetching block header ...")

	var err error
	header, err = debug.GetAccessAPIBlockHeader(ctx, flowClient.RPCClient(), blockID)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to fetch block header")
	}

	log.Info().Msgf(
		"Fetched block header: %s is at height %d",
		blockID,
		header.Height,
	)

	return
}

func SubscribeBlockHeadersFromStartBlockID(
	ctx context.Context,
	flowClient *client.Client,
	startBlockID flow.Identifier,
	blockStatus flow.BlockStatus,
) (get func() (*flow.Header, error)) {
	log.Info().Msg("Subscribing to block headers ...")

	var err error
	get, err = debug.SubscribeAccessAPIBlockHeadersFromStartBlockID(
		ctx,
		flowClient.RPCClient(),
		startBlockID,
		blockStatus,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to subscribe to block headers")
	}

	log.Info().Msg("Subscribed to block headers")

	return
}

func SubscribeBlockHeadersFromLatest(
	ctx context.Context,
	flowClient *client.Client,
	blockStatus flow.BlockStatus,
) (get func() (*flow.Header, error)) {
	log.Info().Msg("Subscribing to block headers ...")

	var err error
	get, err = debug.SubscribeAccessAPIBlockHeadersFromLatest(
		ctx,
		flowClient.RPCClient(),
		blockStatus,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to subscribe to block headers")
	}

	log.Info().Msg("Subscribed to block headers")

	return
}

func FetchBlockTransactions(
	ctx context.Context,
	blockID flow.Identifier,
	flowClient *client.Client,
) []*sdk.Transaction {
	var err error

	log.Info().Msgf("Fetching transactions of block %s ...", blockID)

	blockTransactions, err := flowClient.GetTransactionsByBlockID(ctx, sdk.Identifier(blockID))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to fetch transactions of block")
	}

	for _, blockTx := range blockTransactions {
		log.Info().Msgf("Block transaction: %s", blockTx.ID())
	}

	return blockTransactions
}

func RunBlock(
	blockSnapshot debug.UpdatableStorageSnapshot,
	blockHeader *flow.Header,
	blockTransactions []*sdk.Transaction,
	debuggedTxID flow.Identifier,
	chain flow.Chain,
	wrapTxSnapshot func(blockTxID flow.Identifier, snapshot debug.UpdatableStorageSnapshot) debug.UpdatableStorageSnapshot,
	newSpanExporter func(blockTxID flow.Identifier) otelTrace.SpanExporter,
	computeLimit uint64,
	additionalFVMOptions []fvm.Option,
) (
	results []debug.Result,
) {
	for _, blockTx := range blockTransactions {

		blockTxID := flow.Identifier(blockTx.ID())

		var spanExporter otelTrace.SpanExporter
		if newSpanExporter != nil {
			spanExporter = newSpanExporter(blockTxID)
		}

		txSnapshot := blockSnapshot
		if wrapTxSnapshot != nil {
			txSnapshot = wrapTxSnapshot(blockTxID, blockSnapshot)
		}

		result := RunTransaction(
			blockTx,
			txSnapshot,
			blockHeader,
			chain,
			spanExporter,
			computeLimit,
			additionalFVMOptions,
		)

		updatedRegisters := result.Snapshot.UpdatedRegisters()
		for _, updatedRegister := range updatedRegisters {
			txSnapshot.Set(
				updatedRegister.Key,
				updatedRegister.Value,
			)
		}

		results = append(results, result)

		// Ignore remaining transactions if a specific transaction is being debugged
		if blockTxID == debuggedTxID {
			break
		}
	}

	return
}

func RunTransaction(
	tx *sdk.Transaction,
	snapshot debug.StorageSnapshot,
	header *flow.Header,
	chain flow.Chain,
	spanExporter otelTrace.SpanExporter,
	computeLimit uint64,
	additionalFVMOptions []fvm.Option,
) debug.Result {

	log := log.With().
		Str("tx", tx.ID().String()).
		Logger()

	fvmOptions := []fvm.Option{
		fvm.WithComputationLimit(computeLimit),
	}

	if spanExporter != nil {

		const sync = true
		tracer, err := trace.NewTracerWithExporter(
			log,
			"debug-tx",
			string(chain.ChainID()),
			trace.SensitivityCaptureAll,
			spanExporter,
			sync,
		)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create tracer")
		}

		span, _ := tracer.StartTransactionSpan(context.TODO(), flow.Identifier(tx.ID()), "")
		defer span.End()

		fvmOptions = append(
			fvmOptions,
			fvm.WithTracer(tracer),
			fvm.WithSpan(span),
		)
	}

	fvmOptions = append(fvmOptions, additionalFVMOptions...)

	debugger := debug.NewRemoteDebugger(
		chain,
		log,
		fvmOptions...,
	)

	log.Info().Msgf("Running transaction ...")

	result, err := debugger.RunSDKTransaction(
		tx,
		snapshot,
		header,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Transaction execution failed")
	}

	// TransactionInvoker already logs error
	if result.Output.Err == nil {
		log.Info().Msg("Transaction succeeded")
	} else {
		log.Err(result.Output.Err).Msgf("Transaction failed")
	}

	return result
}

// BlockHashEntropyProvider implements environment.EntropyProvider
// which provides a source of entropy to fvm context (required for Cadence's randomness),
// by using the given block hash.
type BlockHashEntropyProvider struct {
	BlockHash flow.Identifier
}

func (p BlockHashEntropyProvider) RandomSource() ([]byte, error) {
	return p.BlockHash[:], nil
}
