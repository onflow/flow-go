package debug_tx

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	sdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/grpcclient"
	"github.com/onflow/flow-go/utils/debug"
)

// use the following command to forward port 9000 from the EN to localhost:9001
// `gcloud compute ssh '--ssh-flag=-A' --no-user-output-enabled --tunnel-through-iap migrationmainnet1-execution-001 --project flow-multi-region -- -NL 9001:localhost:9000`

var (
	flagAccessAddress       string
	flagExecutionAddress    string
	flagChain               string
	flagTx                  string
	flagComputeLimit        uint64
	flagProposalKeySeq      uint64
	flagUseExecutionDataAPI bool
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

	Cmd.Flags().StringVar(&flagTx, "tx", "", "transaction ID")
	_ = Cmd.MarkFlagRequired("tx")

	Cmd.Flags().Uint64Var(&flagComputeLimit, "compute-limit", 9999, "transaction compute limit")

	Cmd.Flags().Uint64Var(&flagProposalKeySeq, "proposal-key-seq", 0, "proposal key sequence number")

	Cmd.Flags().BoolVar(&flagUseExecutionDataAPI, "use-execution-data-api", false, "use the execution data API")
}

func run(*cobra.Command, []string) {

	chainID := flow.ChainID(flagChain)
	chain := chainID.Chain()

	txID, err := flow.HexStringToIdentifier(flagTx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse transaction ID")
	}

	config, err := grpcclient.NewFlowClientConfig(flagAccessAddress, "", flow.ZeroID, true)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create flow client config")
	}

	flowClient, err := grpcclient.FlowClient(config)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create flow client")
	}

	log.Info().Msg("Fetching transaction ...")

	tx, err := flowClient.GetTransaction(context.Background(), sdk.Identifier(txID))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to fetch transaction")
	}

	log.Info().Msgf("Fetched transaction: %s", tx.ID())

	log.Info().Msg("Fetching transaction result ...")

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

	var snap snapshot.StorageSnapshot

	if flagUseExecutionDataAPI {
		snap, err = debug.NewExecutionDataStorageSnapshot(flowClient.ExecutionDataRPCClient(), nil, blockHeight)
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
		snap, err = debug.NewExecutionNodeStorageSnapshot(executionClient, nil, blockID)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create storage snapshot")
		}
	}

	log.Info().Msg("Debugging transaction ...")

	debugger := debug.NewRemoteDebugger(chain, log.Logger)

	txBody := flow.NewTransactionBody().
		SetScript(tx.Script).
		SetComputeLimit(flagComputeLimit).
		SetPayer(flow.Address(tx.Payer))

	for _, argument := range tx.Arguments {
		txBody.AddArgument(argument)
	}

	for _, authorizer := range tx.Authorizers {
		txBody.AddAuthorizer(flow.Address(authorizer))
	}

	proposalKeySequenceNumber := tx.ProposalKey.SequenceNumber
	if flagProposalKeySeq != 0 {
		proposalKeySequenceNumber = flagProposalKeySeq
	}

	txBody.SetProposalKey(
		flow.Address(tx.ProposalKey.Address),
		tx.ProposalKey.KeyIndex,
		proposalKeySequenceNumber,
	)

	txErr, processErr := debugger.RunTransaction(txBody, snap, header)
	if txErr != nil {
		log.Fatal().Err(txErr).Msg("transaction error")
	}
	if processErr != nil {
		log.Fatal().Err(processErr).Msg("process error")
	}
}
