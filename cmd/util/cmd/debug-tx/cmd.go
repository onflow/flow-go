package debug_tx

import (
	"context"
	"os"

	sdk "github.com/onflow/flow-go-sdk"
	client "github.com/onflow/flow-go-sdk/access/grpc"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"

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

	Cmd.Flags().Uint64Var(&flagComputeLimit, "compute-limit", 9999, "transaction compute limit")

	Cmd.Flags().BoolVar(&flagUseExecutionDataAPI, "use-execution-data-api", true, "use the execution data API (default: true)")

	Cmd.Flags().BoolVar(&flagShowResult, "show-result", false, "show result (default: false)")

	Cmd.Flags().StringVar(&flagBlockID, "block-id", "", "block ID")

	Cmd.Flags().BoolVar(&flagUseVM, "use-vm", false, "use the VM for transaction execution (default: false)")

	Cmd.Flags().StringVar(&flagTracePath, "trace", "", "enable tracing to given path")
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

	if flagBlockID != "" {

		if len(args) != 0 {
			log.Fatal().Msg("cannot provide both block ID and transaction IDs")
		}

		// Block ID provided, fetch the block and its transaction IDs

		blockID, err := flow.HexStringToIdentifier(flagBlockID)
		if err != nil {
			log.Fatal().Err(err).Str("ID", flagBlockID).Msg("failed to parse block ID")
		}

		block, err := flowClient.GetBlockByID(context.Background(), sdk.Identifier(blockID))
		if err != nil {
			log.Fatal().Err(err).Str("ID", flagBlockID).Msg("failed to fetch block by ID")
		}

		runBlockID(
			remoteClient,
			blockID,
			block.Height,
			flow.ZeroID,
			flowClient,
			chain,
			flagUseVM,
		)

	} else {
		// No block ID provided, proceed with transaction IDs from args

		for _, rawTxID := range args {
			txID, err := flow.HexStringToIdentifier(rawTxID)
			if err != nil {
				log.Fatal().Err(err).Str("ID", rawTxID).Msg("failed to parse transaction ID")
			}

			runTransactionID(
				remoteClient,
				txID,
				flowClient,
				chain,
				flagUseVM,
			)
		}
	}
}

func runTransactionID(
	remoteClient debug.RemoteClient,
	txID flow.Identifier,
	flowClient *client.Client,
	chain flow.Chain,
	useVM bool,
) {
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

	runBlockID(
		remoteClient,
		blockID,
		blockHeight,
		txID,
		flowClient,
		chain,
		useVM,
	)
}

func runBlockID(
	remoteClient debug.RemoteClient,
	blockID flow.Identifier,
	blockHeight uint64,
	txID flow.Identifier,
	flowClient *client.Client,
	chain flow.Chain,
	useVM bool,
) {
	log.Info().Msgf("Fetching transactions of block %s ...", blockID)

	blockTransactions, err := flowClient.GetTransactionsByBlockID(context.Background(), sdk.Identifier(blockID))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to fetch transactions of block")
	}

	for _, blockTx := range blockTransactions {
		log.Info().Msgf("Block transaction: %s", blockTx.ID())
	}

	log.Info().Msg("Fetching block header ...")

	header, err := debug.GetAccessAPIBlockHeader(context.Background(), flowClient.RPCClient(), blockID)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to fetch block header")
	}

	log.Info().Msgf(
		"Fetched block header: %s (height %d)",
		header.ID(),
		header.Height,
	)

	log.Info().Msg("Fetching system transaction ...")

	systemTx, err := flowClient.GetSystemTransaction(context.Background(), sdk.Identifier(blockID))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to fetch system transaction")
	}

	log.Info().Msgf("Fetched system transaction: %s", systemTx.ID())

	remoteSnapshot, err := remoteClient.StorageSnapshot(blockHeight, blockID)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create storage snapshot")
	}

	blockSnapshot := debug.NewCachingStorageSnapshot(remoteSnapshot)

	var fvmOptions []fvm.Option

	if flagTracePath != "" {

		var traceFile *os.File
		if flagTracePath == "-" {
			traceFile = os.Stdout
		} else {
			traceFile, err = os.Create(flagTracePath)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to create trace file")
			}
			defer traceFile.Close()
		}

		exporter, err := stdouttrace.New(
			stdouttrace.WithWriter(traceFile),
		)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create trace exporter")
		}

		tracer, err := trace.NewTracerWithExporter(
			log.Logger,
			"debug-tx",
			flagChain,
			trace.SensitivityCaptureAll,
			exporter,
		)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create tracer")
		}

		span, _ := tracer.StartTransactionSpan(context.TODO(), txID, "")
		defer span.End()

		fvmOptions = append(
			fvmOptions,
			fvm.WithTracer(tracer),
			fvm.WithSpan(span),
		)
	}

	debugger := debug.NewRemoteDebugger(
		chain,
		log.Logger,
		useVM,
		useVM,
		fvmOptions...,
	)

	for _, blockTx := range blockTransactions {

		// TODO: add support for executing system transactions
		if blockTx.ID() == systemTx.ID() {
			log.Info().Msg("Skipping system transaction")
			continue
		}

		blockTxID := flow.Identifier(blockTx.ID())

		isDebuggedTx := blockTxID == txID

		result := runTransaction(
			debugger,
			blockTxID,
			flowClient,
			blockSnapshot,
			header,
			isDebuggedTx,
		)

		if flagShowResult && (isDebuggedTx || txID == flow.ZeroID) {
			debug.WriteResult(os.Stdout, blockTxID, result)
		}

		if isDebuggedTx {
			break
		}
	}
}

func runTransaction(
	debugger *debug.RemoteDebugger,
	txID flow.Identifier,
	flowClient *client.Client,
	snapshot *debug.CachingStorageSnapshot,
	header *flow.Header,
	isDebuggedTx bool,
) debug.Result {
	var prefix string
	if !isDebuggedTx {
		prefix = "(prerequisite) "
	}

	log.Info().Msgf("Fetching %stransaction %s ...", prefix, txID)

	tx, err := flowClient.GetTransaction(context.Background(), sdk.Identifier(txID))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to fetch transaction")
	}

	log.Info().Msgf("Fetched %stransaction: %s", prefix, tx.ID())

	result, err := debugger.RunSDKTransaction(tx, snapshot, header, flagComputeLimit)
	if err != nil {
		log.Fatal().Err(err).Msg("Transaction execution failed")
	}

	if result.Output.Err != nil {
		log.Err(result.Output.Err).Msg("Transaction failed")
	} else {
		log.Info().Msg("Transaction succeeded")
	}

	return result
}
