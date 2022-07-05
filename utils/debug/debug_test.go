package debug_test

import (
	"context"
	"fmt"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/debug"
)

func TestDebugger_RunTransaction(t *testing.T) {
	t.Skip()

	enGrpcAddress := "----"
	anGrpcAddress := "access.mainnet.nodes.onflow.org:9000"
	chain := flow.Mainnet.Chain()
	transactionHex := "8492c79018e2628f0e9fd9a30995eadf55c1607b03cc2d78d405997487052c2c"

	txID, err := flow.HexStringToIdentifier(transactionHex)
	require.NoError(t, err)

	txErr, err := rerunTransaction(t,
		enGrpcAddress,
		anGrpcAddress,
		chain,
		txID,
	)
	require.NoError(t, txErr)
	require.NoError(t, err)
}

// rerunTransaction gets the transaction and arguments from the access node
// Creates a new transaction with that data
// And runs it at the latest block
func rerunTransaction(
	t *testing.T,
	enGRPCAddress string,
	anGRPCAddress string,
	chain flow.Chain,
	transactionId flow.Identifier) (txErr, processError error) {

	ctx := context.Background()

	// get existing transaction data
	anClient, err := client.New(anGRPCAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	latestHeader, err := anClient.GetLatestBlockHeader(ctx, true)
	require.NoError(t, err)
	tx, err := anClient.GetTransaction(ctx, sdk.Identifier(transactionId))
	require.NoError(t, err)
	//txResult, err := anClient.GetTransactionResult(ctx, sdk.Identifier(transactionId))
	//require.NoError(t, err)

	// construct new transaction
	txBody := flow.NewTransactionBody().
		SetGasLimit(tx.GasLimit).
		SetScript(tx.Script).
		SetPayer(flow.Address(tx.Payer)).
		SetProposalKey(flow.Address(tx.ProposalKey.Address), uint64(tx.ProposalKey.KeyIndex), tx.ProposalKey.SequenceNumber).
		SetArguments(tx.Arguments)
	for _, a := range tx.Authorizers {
		txBody.AddAuthorizer(flow.Address(a))
	}

	debugger := debug.NewRemoteDebugger(enGRPCAddress, chain, zerolog.New(os.Stdout).With().Logger())

	testCacheFile := fmt.Sprintf("%d@%s.cache", latestHeader.Height, transactionId)

	txErr, processError, debugInfo := debugger.RunTransactionAtBlockID(txBody, flow.Identifier(latestHeader.ID), testCacheFile)

	for _, r := range debugInfo.RegistersReadCollapseSlabReads() {
		println(r.Owner, r.Key, r.Size)
	}

	return txErr, processError
}
