package debug_tx

import (
	"context"

	sdk "github.com/onflow/flow-go-sdk"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/grpcclient"
	"github.com/onflow/flow-go/utils/debug"
)

var (
	flagAccessAddress    string
	flagExecutionAddress string
	flagChain            string
	flagTx               string
	flagComputeLimit     uint64
	flagAtLatestBlock    bool
	flagProposalKeySeq   uint64
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

	Cmd.Flags().BoolVar(&flagAtLatestBlock, "at-latest-block", false, "run at latest block")

	Cmd.Flags().Uint64Var(&flagProposalKeySeq, "proposal-key-seq", 0, "proposal key sequence number")
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

	log.Info().Msgf("Fetched transaction result: %s at block %s", txResult.Status, txResult.BlockID)

	log.Info().Msg("Debugging transaction ...")

	debugger := debug.NewRemoteDebugger(
		flagExecutionAddress,
		chain,
		log.Logger,
	)

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

	var txErr, processErr error
	if flagAtLatestBlock {
		txErr, processErr = debugger.RunTransaction(txBody)
	} else {
		txErr, processErr = debugger.RunTransactionAtBlockID(
			txBody,
			flow.Identifier(txResult.BlockID),
			"",
		)
	}
	if txErr != nil {
		log.Fatal().Err(txErr).Msg("transaction error")
	}
	if processErr != nil {
		log.Fatal().Err(processErr).Msg("process error")
	}
}
