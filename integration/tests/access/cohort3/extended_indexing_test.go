package cohort3

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/cadence"
	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/indexes"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	storagepebble "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/utils/unittest"
)

const (
	// sendFlowTokensScript is a Cadence script that transfers Flow tokens to the given address.
	sendFlowTokensScript = `
import FungibleToken from 0x%s
import FlowToken from 0x%s

transaction(amount: UFix64, to: Address) {
    let sentVault: @{FungibleToken.Vault}

    prepare(signer: auth(BorrowValue) &Account) {
        let vaultRef = signer.storage.borrow<auth(FungibleToken.Withdraw) &FlowToken.Vault>(from: /storage/flowTokenVault)
            ?? panic("Could not borrow reference to the owner's Vault!")
        self.sentVault <- vaultRef.withdraw(amount: amount)
    }

    execute {
        let receiverRef = getAccount(to)
            .capabilities.borrow<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)
            ?? panic("Could not borrow receiver reference to the recipient's Vault")
        receiverRef.deposit(from: <-self.sentVault)
    }
}
`
)

func TestExtendedIndexing(t *testing.T) {
	suite.Run(t, new(ExtendedIndexingSuite))
}

// ExtendedIndexingSuite verifies that the extended indexer (account_transactions) correctly indexes
// data when enabled on an access node. It uses the gRPC API to confirm that block heights are
// being processed through the full pipeline:
//
//	block production → execution data sync → execution state indexing → extended indexing
type ExtendedIndexingSuite struct {
	suite.Suite
	net    *testnet.FlowNetwork
	cancel context.CancelFunc
}

func (s *ExtendedIndexingSuite) SetupTest() {
	consensusConfigs := []func(config *testnet.NodeConfig){
		testnet.WithAdditionalFlag("--cruise-ctl-fallback-proposal-duration=250ms"),
		testnet.WithAdditionalFlagf("--required-verification-seal-approvals=%d", 1),
		testnet.WithAdditionalFlagf("--required-construction-seal-approvals=%d", 1),
		testnet.WithLogLevel(zerolog.FatalLevel),
	}

	// Access node with execution data sync, execution data indexing, and extended indexing enabled.
	accessNodeOpts := []func(config *testnet.NodeConfig){
		testnet.WithLogLevel(zerolog.InfoLevel),
		testnet.WithAdditionalFlag("--execution-data-sync-enabled=true"),
		testnet.WithAdditionalFlag("--execution-data-indexing-enabled=true"),
		testnet.WithAdditionalFlagf("--execution-data-dir=%s", testnet.DefaultExecutionDataServiceDir),
		testnet.WithAdditionalFlagf("--execution-state-dir=%s", testnet.DefaultExecutionStateDir),
		testnet.WithAdditionalFlag("--extended-indexing-enabled=true"),
		testnet.WithAdditionalFlag("--extended-indexing-db-dir=/data/indexer"),
	}

	nodeConfigs := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleAccess, accessNodeOpts...),
	}

	conf := testnet.NewNetworkConfig("access_extended_indexing_test", nodeConfigs)
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.net.Start(ctx)
}

func (s *ExtendedIndexingSuite) TearDownTest() {
	if s.net != nil {
		s.net.Remove()
		s.net = nil
	}
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

// TestExtendedIndexerProgresses verifies that the account_transactions extended indexer processes
// blocks successfully. It uses the gRPC API to confirm that the indexer is making progress.
func (s *ExtendedIndexingSuite) TestExtendedIndexerProgresses() {
	targetHeight := uint64(10)

	client, err := s.net.ContainerByName(testnet.PrimaryAN).TestnetClient()
	require.NoError(s.T(), err)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	err = client.WaitUntilIndexed(ctx, targetHeight)
	require.NoError(s.T(), err)
}

// TestAccountTransactionIndexing verifies that the extended indexer correctly indexes account
// transactions by:
//  1. Creating a new account (service account as payer)
//  2. Transferring Flow tokens from the service account to the new account
//  3. Sending a noop transaction from the new account (making it a payer/authorizer)
//  4. Waiting for the indexer to process those blocks
//  5. Stopping the access node and reading the index DB directly
//  6. Verifying that both accounts have the expected transaction entries
func (s *ExtendedIndexingSuite) TestAccountTransactionIndexing() {
	t := s.T()
	ctx := context.Background()

	accessAddr := s.net.ContainerByName(testnet.PrimaryAN).Addr(testnet.GRPCPort)

	// Step 1: Get a testnet client for the service account
	serviceClient, err := s.net.ContainerByName(testnet.PrimaryAN).TestnetClient()
	require.NoError(t, err)

	latestBlockID, err := serviceClient.GetLatestBlockID(ctx)
	require.NoError(t, err)

	// Step 2: Create a new account
	accountPrivateKey := lib.RandomPrivateKey()
	accountKey := sdk.NewAccountKey().
		FromPrivateKey(accountPrivateKey).
		SetHashAlgo(sdkcrypto.SHA3_256).
		SetWeight(sdk.AccountKeyWeightThreshold)

	newAccountAddress, err := serviceClient.CreateAccount(ctx, accountKey, sdk.Identifier(latestBlockID))
	require.NoError(t, err)
	t.Logf("created new account: %s", newAccountAddress)

	// Step 3: Transfer Flow tokens from the service account to the new account (to fund it)
	latestBlockID, err = serviceClient.GetLatestBlockID(ctx)
	require.NoError(t, err)

	transferTx := s.buildFlowTransferTx(newAccountAddress, "1.0")
	transferTx.
		SetReferenceBlockID(sdk.Identifier(latestBlockID)).
		SetProposalKey(serviceClient.SDKServiceAddress(), 0, serviceClient.GetAndIncrementSeqNumber()).
		SetPayer(serviceClient.SDKServiceAddress()).
		SetComputeLimit(9999)

	err = serviceClient.SignAndSendTransaction(ctx, transferTx)
	require.NoError(t, err)

	transferTxResult, err := serviceClient.WaitForSealed(ctx, transferTx.ID())
	require.NoError(t, err)
	require.NoError(t, transferTxResult.Error)
	t.Logf("transfer tx sealed at height %d, tx ID: %s", transferTxResult.BlockHeight, transferTx.ID())

	// Step 4: Send a noop transaction from the new account (so it's indexed as payer/authorizer)
	newAccountClient, err := testnet.NewClientWithKey(
		accessAddr, newAccountAddress, accountPrivateKey, flow.Localnet.Chain(),
	)
	require.NoError(t, err)

	latestBlockID, err = newAccountClient.GetLatestBlockID(ctx)
	require.NoError(t, err)

	noopTx := sdk.NewTransaction().
		SetScript(unittest.NoopTxScript()).
		SetReferenceBlockID(sdk.Identifier(latestBlockID)).
		SetProposalKey(newAccountAddress, 0, newAccountClient.GetAndIncrementSeqNumber()).
		SetPayer(newAccountAddress).
		SetComputeLimit(9999)

	err = newAccountClient.SignAndSendTransaction(ctx, noopTx)
	require.NoError(t, err)

	noopTxResult, err := newAccountClient.WaitForSealed(ctx, noopTx.ID())
	require.NoError(t, err)
	require.NoError(t, noopTxResult.Error)
	t.Logf("noop tx sealed at height %d, tx ID: %s", noopTxResult.BlockHeight, noopTx.ID())

	// Step 5: Wait for the extended indexer to process these blocks
	waitCtx, waitCancel := context.WithTimeout(ctx, 120*time.Second)
	defer waitCancel()

	// Wait for a few blocks past the target to give the extended indexer time to process.
	err = serviceClient.WaitUntilIndexed(waitCtx, noopTxResult.BlockHeight+10)
	require.NoError(t, err)

	// Step 6: Stop the access node so we can safely open its DB
	err = s.net.StopContainerByName(ctx, testnet.PrimaryAN)
	require.NoError(t, err)

	// Step 7: Open the extended indexer Pebble DB from the host
	accessContainer := s.net.ContainerByName(testnet.PrimaryAN)
	indexerDBPath := filepath.Join(filepath.Dir(accessContainer.DBPath()), "indexer")
	t.Logf("opening indexer DB at: %s", indexerDBPath)

	pdb, err := storagepebble.SafeOpen(unittest.Logger(), indexerDBPath)
	require.NoError(t, err)
	defer pdb.Close()

	db := pebbleimpl.ToDB(pdb)

	accountTxIndex, err := indexes.NewAccountTransactions(db)
	require.NoError(t, err)

	serviceAddr := flow.Address(serviceClient.SDKServiceAddress())
	newAddr := flow.Address(newAccountAddress)

	// Step 8: Verify account transactions for the service account
	serviceAccountTxs, err := accountTxIndex.TransactionsByAddress(
		serviceAddr,
		accountTxIndex.FirstIndexedHeight(),
		accountTxIndex.LatestIndexedHeight(),
	)
	require.NoError(t, err)
	t.Logf("service account has %d indexed transactions", len(serviceAccountTxs))

	// The service account should have entries for the transfer tx (as payer/proposer/authorizer).
	transferTxID := flow.Identifier(transferTx.ID())
	foundTransferForService := false
	for _, entry := range serviceAccountTxs {
		if entry.TransactionID == transferTxID {
			foundTransferForService = true
			s.Contains(entry.Roles, access.TransactionRoleAuthorizer, "service account should be authorizer for transfer tx")
			s.Contains(entry.Roles, access.TransactionRolePayer, "service account should be payer for transfer tx")
			s.Contains(entry.Roles, access.TransactionRoleProposer, "service account should be proposer for transfer tx")
			s.Equal(transferTxResult.BlockHeight, entry.BlockHeight)
			break
		}
	}
	s.True(foundTransferForService, "transfer tx not found in service account's indexed transactions")

	// Step 9: Verify account transactions for the new account
	newAccountTxs, err := accountTxIndex.TransactionsByAddress(
		newAddr,
		accountTxIndex.FirstIndexedHeight(),
		accountTxIndex.LatestIndexedHeight(),
	)
	require.NoError(t, err)
	t.Logf("new account has %d indexed transactions", len(newAccountTxs))

	// The new account should have
	// * noop tx (as payer/proposer, not authorizer since the noop script has no prepare block).
	// * transfer tx (not authorizer since the new account received the funds and was only in events).
	noopTxID := flow.Identifier(noopTx.ID())
	foundNoopForNewAccount := false
	foundTransferForNewAccount := false
	for _, entry := range newAccountTxs {
		if entry.TransactionID == noopTxID {
			foundNoopForNewAccount = true
			s.Equal(noopTxResult.BlockHeight, entry.BlockHeight)
			s.NotContains(entry.Roles, access.TransactionRoleAuthorizer, "new account should not be authorizer for noop tx")
			s.Contains(entry.Roles, access.TransactionRoleProposer, "new account should be proposer for noop tx")
			s.Contains(entry.Roles, access.TransactionRolePayer, "new account should be payer for noop tx")
		}
		if entry.TransactionID == transferTxID {
			foundTransferForNewAccount = true
			s.NotContains(entry.Roles, access.TransactionRoleAuthorizer, "new account should not be authorizer for transfer tx")
			s.NotContains(entry.Roles, access.TransactionRoleProposer, "new account should not be proposer for transfer tx")
			s.NotContains(entry.Roles, access.TransactionRolePayer, "new account should not be payer for transfer tx")
			s.Contains(entry.Roles, access.TransactionRoleInteraction, "new account should be interaction for transfer tx")
			s.Equal(transferTxResult.BlockHeight, entry.BlockHeight)
		}
	}
	s.True(foundNoopForNewAccount, "noop tx not found in new account's indexed transactions")
	s.True(foundTransferForNewAccount, "transfer tx not found in new account's indexed transactions")
}

// buildFlowTransferTx constructs a Cadence transaction that transfers Flow tokens to the given address.
func (s *ExtendedIndexingSuite) buildFlowTransferTx(to sdk.Address, amount string) *sdk.Transaction {
	contracts := systemcontracts.SystemContractsForChain(flow.Localnet)
	ftAddr := contracts.FungibleToken.Address.Hex()
	flowTokenAddr := contracts.FlowToken.Address.Hex()

	script := fmt.Sprintf(sendFlowTokensScript, ftAddr, flowTokenAddr)

	amountArg, err := cadence.NewUFix64(amount)
	require.NoError(s.T(), err)
	toArg := cadence.NewAddress(cadence.BytesToAddress(to.Bytes()))

	tx := sdk.NewTransaction().
		SetScript([]byte(strings.TrimSpace(script))).
		AddAuthorizer(sdk.Address(flow.Localnet.Chain().ServiceAddress()))

	err = tx.AddArgument(amountArg)
	require.NoError(s.T(), err)
	err = tx.AddArgument(toArg)
	require.NoError(s.T(), err)

	return tx
}
