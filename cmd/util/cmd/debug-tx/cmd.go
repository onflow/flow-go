package debug_tx

import (
	"cmp"
	"context"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	client "github.com/onflow/flow-go-sdk/access/grpc"
	"github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/onflow/flow/protobuf/go/flow/executiondata"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	sdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
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
	flagProposalKeySeq      uint64
	flagUseExecutionDataAPI bool
	flagDumpRegisters       bool
	flagCollectionID        string
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

	Cmd.Flags().StringVar(&flagExecutionAddress, "execution-address", "", "address of the execution node")
	_ = Cmd.MarkFlagRequired("execution-address")

	Cmd.Flags().Uint64Var(&flagComputeLimit, "compute-limit", 9999, "transaction compute limit")

	Cmd.Flags().Uint64Var(&flagProposalKeySeq, "proposal-key-seq", 0, "proposal key sequence number")

	Cmd.Flags().BoolVar(&flagUseExecutionDataAPI, "use-execution-data-api", false, "use the execution data API")

	Cmd.Flags().BoolVar(&flagDumpRegisters, "dump-registers", false, "dump registers")

	Cmd.Flags().StringVar(&flagCollectionID, "collection-id", "", "collection ID")

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

	if flagCollectionID != "" {
		if flagBlockID != "" {
			log.Fatal().Msg("Cannot specify both collection ID and block ID")
		}

		// Collection ID provided, fetch the collection and its transaction IDs

		colID, err := flow.HexStringToIdentifier(flagCollectionID)
		if err != nil {
			log.Fatal().Err(err).Str("ID", flagCollectionID).Msg("failed to parse collection ID")
		}

		col, err := flowClient.GetCollectionByID(context.Background(), sdk.Identifier(colID))
		if err != nil {
			log.Fatal().Err(err).Str("ID", flagCollectionID).Msg("failed to fetch collection by ID")
		}

		var rawTxIDs []string
		for _, txID := range col.TransactionIDs {
			rawTxIDs = append(rawTxIDs, txID.String())
		}
		log.Info().Msgf("Fetched collection: %s. Transaction IDs: %s", col.ID(), strings.Join(rawTxIDs, " "))

		if len(args) > 0 {
			log.Warn().Msg("Collection ID provided, transaction IDs from args will be ignored!")
		}

		for _, txID := range col.TransactionIDs {
			runTransactionID(flow.Identifier(txID), flowClient, chain)
		}

	} else if flagBlockID != "" {
		if flagCollectionID != "" {
			log.Fatal().Msg("Cannot specify both collection ID and block ID")
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
			blockID,
			block.Height,
			flow.ZeroID,
			flowClient,
			chain,
		)

	} else {

		// No collection ID provided, proceed with transaction IDs from args

		for _, rawTxID := range args {
			txID, err := flow.HexStringToIdentifier(rawTxID)
			if err != nil {
				log.Fatal().Err(err).Str("ID", rawTxID).Msg("failed to parse transaction ID")
			}

			runTransactionID(txID, flowClient, chain)
		}
	}
}

func runTransactionID(txID flow.Identifier, flowClient *client.Client, chain flow.Chain) {
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
		blockID,
		blockHeight,
		txID,
		flowClient,
		chain,
	)
}

func runBlockID(
	blockID flow.Identifier,
	blockHeight uint64,
	txID flow.Identifier,
	flowClient *client.Client,
	chain flow.Chain,
) {
	log.Info().Msgf("Fetching transactions of block %s ...", blockID)

	txsResult, err := flowClient.GetTransactionsByBlockID(context.Background(), sdk.Identifier(blockID))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to fetch transactions of block")
	}

	for _, blockTx := range txsResult {
		log.Info().Msgf("Block transaction: %s", blockTx.ID())
	}

	log.Info().Msg("Fetching block header ...")

	header, err := debug.GetAccessAPIBlockHeader(flowClient.RPCClient(), context.Background(), blockID)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to fetch block header")
	}

	log.Info().Msgf(
		"Fetched block header: %s (height %d)",
		header.ID(),
		header.Height,
	)

	var remoteSnapshot snapshot.StorageSnapshot

	if flagUseExecutionDataAPI {
		accessConn, err := grpc.NewClient(
			flagAccessAddress,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create access connection")
		}
		defer accessConn.Close()

		executionDataClient := executiondata.NewExecutionDataAPIClient(accessConn)

		// The execution data API provides the *resulting* data,
		// so fetch the data for the parent block for the *initial* data.
		remoteSnapshot, err = debug.NewExecutionDataStorageSnapshot(executionDataClient, nil, blockHeight-1)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create storage snapshot")
		}
	} else {
		executionConn, err := grpc.NewClient(
			flagExecutionAddress,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create execution connection")
		}
		defer executionConn.Close()

		executionClient := execution.NewExecutionAPIClient(executionConn)

		remoteSnapshot, err = debug.NewExecutionNodeStorageSnapshot(executionClient, nil, blockID)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create storage snapshot")
		}
	}

	blockSnapshot := newBlockSnapshot(remoteSnapshot)

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
    flagUseVM,
    flagUseVM,
		fvmOptions...,
	)

	for _, blockTx := range txsResult {
		blockTxID := flow.Identifier(blockTx.ID())

		isDebuggedTx := blockTxID == txID

		dumpRegisters := flagDumpRegisters && (isDebuggedTx || txID == flow.ZeroID)

		runTransaction(
			debugger,
			blockTxID,
			flowClient,
			blockSnapshot,
			header,
			dumpRegisters,
		)

		if isDebuggedTx {
			break
		}
	}
}

func runTransaction(
	debugger *debug.RemoteDebugger,
	txID flow.Identifier,
	flowClient *client.Client,
	blockSnapshot *blockSnapshot,
	header *flow.Header,
	dumpRegisters bool,
) {

	log.Info().Msgf("Fetching transaction %s ...", txID)

	tx, err := flowClient.GetTransaction(context.Background(), sdk.Identifier(txID))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to fetch transaction")
	}

	log.Info().Msgf("Fetched transaction: %s", tx.ID())

	log.Info().Msgf("Debugging transaction %s ...", tx.ID())

	txBodyBuilder := flow.NewTransactionBodyBuilder().
		SetScript(tx.Script).
		SetComputeLimit(flagComputeLimit).
		SetPayer(flow.Address(tx.Payer))

	for _, argument := range tx.Arguments {
		txBodyBuilder.AddArgument(argument)
	}

	for _, authorizer := range tx.Authorizers {
		txBodyBuilder.AddAuthorizer(flow.Address(authorizer))
	}

	proposalKeySequenceNumber := tx.ProposalKey.SequenceNumber
	if flagProposalKeySeq != 0 {
		proposalKeySequenceNumber = flagProposalKeySeq
	}

	txBodyBuilder.SetProposalKey(
		flow.Address(tx.ProposalKey.Address),
		tx.ProposalKey.KeyIndex,
		proposalKeySequenceNumber,
	)

	txBody, err := txBodyBuilder.Build()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to build transaction body")
	}

	resultSnapshot, txErr, processErr := debugger.RunTransaction(
		txBody,
		blockSnapshot,
		header,
	)
	if processErr != nil {
		log.Fatal().Err(processErr).Msg("Failed to process transaction")
	}

	if txErr != nil {
		log.Err(txErr).Msg("Transaction failed")
	} else {
		log.Info().Msg("Transaction succeeded")
	}

	updatedRegisters := resultSnapshot.UpdatedRegisters()
	for _, updatedRegister := range updatedRegisters {
		blockSnapshot.Set(
			updatedRegister.Key,
			updatedRegister.Value,
		)
	}

	if dumpRegisters {
		dumpReadRegisters(txID, resultSnapshot.ReadRegisterIDs())
		dumpUpdatedRegisters(txID, updatedRegisters)
	}
}

func dumpReadRegisters(txID flow.Identifier, readRegisterIDs []flow.RegisterID) {
	filename := fmt.Sprintf("%s.reads.csv", txID)
	file, err := os.Create(filename)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to create reads file: %s", filename)
	}
	defer file.Close()

	sortRegisterIDs(readRegisterIDs)

	writer := csv.NewWriter(file)
	defer writer.Flush()

	err = writer.Write([]string{"RegisterID"})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to write header")
	}

	for _, readRegisterID := range readRegisterIDs {
		err = writer.Write([]string{
			readRegisterID.String(),
		})
		if err != nil {
			log.Fatal().Err(err).Msgf("Failed to write read register: %s", readRegisterID)
		}
	}
}

func dumpUpdatedRegisters(txID flow.Identifier, updatedRegisters []flow.RegisterEntry) {
	filename := fmt.Sprintf("%s.updates.csv", txID)
	file, err := os.Create(filename)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to create writes file: %s", filename)
	}
	defer file.Close()

	sortRegisterEntries(updatedRegisters)

	writer := csv.NewWriter(file)
	defer writer.Flush()

	err = writer.Write([]string{"RegisterID", "Value"})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to write header")
	}

	for _, updatedRegister := range updatedRegisters {
		err = writer.Write([]string{
			updatedRegister.Key.String(),
			hex.EncodeToString(updatedRegister.Value),
		})
		if err != nil {
			log.Fatal().Err(err).Msgf("Failed to write updated register: %s", updatedRegister)
		}
	}
}

func compareRegisterIDs(a flow.RegisterID, b flow.RegisterID) int {
	return cmp.Or(
		cmp.Compare(a.Owner, b.Owner),
		cmp.Compare(a.Key, b.Key),
	)
}

func sortRegisterIDs(registerIDs []flow.RegisterID) {
	slices.SortFunc(registerIDs, func(a, b flow.RegisterID) int {
		return compareRegisterIDs(a, b)
	})
}

func sortRegisterEntries(registerEntries []flow.RegisterEntry) {
	slices.SortFunc(registerEntries, func(a, b flow.RegisterEntry) int {
		return compareRegisterIDs(a.Key, b.Key)
	})
}

type blockSnapshot struct {
	cache   *debug.InMemoryRegisterCache
	backing snapshot.StorageSnapshot
}

var _ snapshot.StorageSnapshot = (*blockSnapshot)(nil)

func newBlockSnapshot(backing snapshot.StorageSnapshot) *blockSnapshot {
	cache := debug.NewInMemoryRegisterCache()
	return &blockSnapshot{
		cache:   cache,
		backing: backing,
	}
}

func (s *blockSnapshot) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	data, found := s.cache.Get(id.Key, id.Owner)
	if found {
		return data, nil
	}

	return s.backing.Get(id)
}

func (s *blockSnapshot) Set(id flow.RegisterID, value flow.RegisterValue) {
	s.cache.Set(id.Key, id.Owner, value)
}
