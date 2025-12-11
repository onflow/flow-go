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
	flagUseVM               bool
	flagTracePath           string
	flagLogTraces           bool
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

	Cmd.Flags().BoolVar(&flagUseVM, "use-vm", false, "use the VM for transaction execution (default: false)")

	Cmd.Flags().StringVar(&flagTracePath, "trace", "", "enable tracing to given path")

	Cmd.Flags().BoolVar(&flagLogTraces, "log-cadence-traces", false, "log Cadence traces. requires --trace and --only-trace-cadence to be set (default: false)")

	Cmd.Flags().BoolVar(&flagOnlyTraceCadence, "only-trace-cadence", false, "when tracing, only include spans related to Cadence execution (default: false)")

	Cmd.Flags().StringVar(&flagEntropyProvider, "entropy-provider", "none", "entropy provider to use (default: none; options: none, block-hash)")
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

	var traceFile *os.File
	if flagTracePath == "-" {
		traceFile = os.Stdout
	} else if flagTracePath != "" {
		traceFile, err = os.Create(flagTracePath)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create trace file")
		}
		defer traceFile.Close()
	}

	var spanExporter otelTrace.SpanExporter
	if traceFile != nil {
		if flagOnlyTraceCadence {
			cadenceSpanExporter := &debug.InterestingCadenceSpanExporter{
				Log: flagLogTraces,
			}
			defer func() {
				err = cadenceSpanExporter.WriteSpans(traceFile)
				if err != nil {
					log.Fatal().Err(err).Msg("failed to write spans")
				}
			}()
			spanExporter = cadenceSpanExporter
		} else {
			spanExporter, err = stdouttrace.New(
				stdouttrace.WithWriter(traceFile),
				stdouttrace.WithoutTimestamps(),
			)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to create trace exporter")
			}
		}
	}

	if flagBlockID != "" {

		if len(args) != 0 {
			log.Fatal().Msg("cannot provide both block ID and transaction IDs")
		}

		// Block ID provided, fetch the block and its transaction IDs

		blockID, err := flow.HexStringToIdentifier(flagBlockID)
		if err != nil {
			log.Fatal().Err(err).Str("ID", flagBlockID).Msg("failed to parse block ID")
		}

		header := FetchBlockHeader(blockID, flowClient)
		blockTransactions, systemTxID := FetchBlockTransactions(blockID, flowClient)

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
			systemTxID,
			chain,
			flagUseVM,
			nil,
			newSpanExporter,
			flagComputeLimit,
			fvmOptions(blockID),
		)

		if flagShowResult {
			for i, blockTx := range blockTransactions {
				// Skip system transaction
				if blockTx.ID() == systemTxID {
					continue
				}

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
				log.Fatal().Err(err).Str("ID", rawTxID).Msg("failed to parse transaction ID")
			}

			result := RunSingleTransaction(
				remoteClient,
				txID,
				flowClient,
				chain,
				flagUseVM,
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
	case "block-hash":
		options = append(
			options,
			fvm.WithEntropyProvider(BlockHashEntropyProvider{
				BlockHash: blockID,
			}),
		)
	}

	return options
}

func RunSingleTransaction(
	remoteClient debug.RemoteClient,
	txID flow.Identifier,
	flowClient *client.Client,
	chain flow.Chain,
	useVM bool,
	spanExporter otelTrace.SpanExporter,
	computeLimit uint64,
) debug.Result {
	log.Info().Msgf("Fetching transaction result for %s ...", txID)

	txResult, err := flowClient.GetTransactionResult(context.Background(), sdk.Identifier(txID))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to fetch transaction result")
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
	header := FetchBlockHeader(blockID, flowClient)
	blockTransactions, systemTxID := FetchBlockTransactions(blockID, flowClient)

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
		systemTxID,
		chain,
		useVM,
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

	log.Fatal().Msg("transaction not found in block transactions")

	return debug.Result{}
}

func NewBlockSnapshot(
	remoteClient debug.RemoteClient,
	blockHeader *flow.Header,
) *debug.CachingStorageSnapshot {

	remoteSnapshot, err := remoteClient.StorageSnapshot(blockHeader.Height, blockHeader.ID())
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create storage snapshot")
	}

	return debug.NewCachingStorageSnapshot(remoteSnapshot)
}

func FetchBlockHeader(
	blockID flow.Identifier,
	flowClient *client.Client,
) (header *flow.Header) {
	log.Info().Msg("Fetching block header ...")

	var err error
	header, err = debug.GetAccessAPIBlockHeader(context.Background(), flowClient.RPCClient(), blockID)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to fetch block header")
	}

	log.Info().Msgf(
		"Fetched block header: %s is at height %d",
		blockID,
		header.Height,
	)

	return
}

func FetchBlockTransactions(
	blockID flow.Identifier,
	flowClient *client.Client,
) (
	blockTransactions []*sdk.Transaction,
	systemTxID sdk.Identifier,
) {
	var err error

	log.Info().Msgf("Fetching transactions of block %s ...", blockID)

	blockTransactions, err = flowClient.GetTransactionsByBlockID(context.Background(), sdk.Identifier(blockID))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to fetch transactions of block")
	}

	for _, blockTx := range blockTransactions {
		log.Info().Msgf("Block transaction: %s", blockTx.ID())
	}

	log.Info().Msg("Fetching system transaction ...")

	systemTx, err := flowClient.GetSystemTransaction(context.Background(), sdk.Identifier(blockID))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to fetch system transaction")
	}

	systemTxID = systemTx.ID()
	log.Info().Msgf("Fetched system transaction: %s", systemTxID)

	return
}

func RunBlock(
	blockSnapshot debug.UpdatableStorageSnapshot,
	blockHeader *flow.Header,
	blockTransactions []*sdk.Transaction,
	debuggedTxID flow.Identifier,
	systemTxID sdk.Identifier,
	chain flow.Chain,
	useVM bool,
	wrapTxSnapshot func(blockTxID flow.Identifier, snapshot debug.UpdatableStorageSnapshot) debug.UpdatableStorageSnapshot,
	newSpanExporter func(blockTxID flow.Identifier) otelTrace.SpanExporter,
	computeLimit uint64,
	additionalFVMOptions []fvm.Option,
) (
	results []debug.Result,
) {
	for _, blockTx := range blockTransactions {

		// TODO: add support for executing system transactions
		if blockTx.ID() == systemTxID {
			log.Info().Msg("Skipping system transaction")
			continue
		}

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
			useVM,
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
	useVM bool,
	spanExporter otelTrace.SpanExporter,
	computeLimit uint64,
	additionalFVMOptions []fvm.Option,
) debug.Result {

	log := log.With().
		Str("tx", tx.ID().String()).
		Logger()

	fvmOptions := []fvm.Option{
		fvm.WithCadenceVMEnabled(useVM),
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
			log.Fatal().Err(err).Msg("failed to create tracer")
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
