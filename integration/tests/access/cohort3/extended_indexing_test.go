package cohort3

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/antihax/optional"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/cadence"
	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	swagger "github.com/onflow/flow/openapi/experimental/go-client-generated"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
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

type transactionExpectation struct {
	txID            flow.Identifier
	address         flow.Address
	expectedRoles   []string
	unexpectedRoles []string
}

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
	net         *testnet.FlowNetwork
	cancel      context.CancelFunc
	apiClient   *testnet.ExperimentalAPIClient
	restBaseURL string
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
		testnet.WithAdditionalFlagf("--extended-indexing-db-dir=%s", testnet.DefaultExtendedIndexingDir),
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

	restPort := s.net.ContainerByName(testnet.PrimaryAN).Port(testnet.RESTPort)
	s.restBaseURL = fmt.Sprintf("http://localhost:%s", restPort)

	apiClient, err := testnet.NewExperimentalAPIClient(s.restBaseURL)
	s.Require().NoError(err)
	s.apiClient = apiClient
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

// TestExtendedIndexing verifies the REST API endpoint for querying account transactions.
// It exercises the full pipeline: transaction submission → indexing → REST API response, including
// pagination and role filtering.
func (s *ExtendedIndexingSuite) TestExtendedIndexing() {
	expectations := s.runTransactions()
	for _, expectation := range expectations {
		s.verifyAccountTransactionRoles(expectation.address.String(), expectation.txID.String(), expectation.expectedRoles, expectation.unexpectedRoles)
	}

	// Verify that transaction bodies and results are populated and match the standard REST API
	s.verifyTransactionDetailsFromAPI(expectations)

	serviceClient, err := s.net.ContainerByName(testnet.PrimaryAN).TestnetClient()
	s.Require().NoError(err)
	serviceAddr := flow.Address(serviceClient.SDKServiceAddress())

	// Verify API pagination
	s.verifyPagination(serviceAddr.String())

	// Verify API role filtering
	s.verifyRoleFiltering(serviceAddr.String())
}

func (s *ExtendedIndexingSuite) runTransactions() []transactionExpectation {
	ctx := context.Background()

	// Step 1: Get a testnet client for the service account
	serviceClient, err := s.net.ContainerByName(testnet.PrimaryAN).TestnetClient()
	s.Require().NoError(err)

	latestBlockID, err := serviceClient.GetLatestBlockID(ctx)
	s.Require().NoError(err)

	// Step 2: Create a new account
	accountPrivateKey := lib.RandomPrivateKey()
	accountKey := sdk.NewAccountKey().
		FromPrivateKey(accountPrivateKey).
		SetHashAlgo(sdkcrypto.SHA3_256).
		SetWeight(sdk.AccountKeyWeightThreshold)

	newAccountAddress, createTxResult, err := serviceClient.CreateAccount(ctx, accountKey, sdk.Identifier(latestBlockID))
	s.Require().NoError(err)
	createAccountTxID := flow.Identifier(createTxResult.TransactionID)
	s.T().Logf("created new account: %s", newAccountAddress)

	// Step 3: Transfer Flow tokens to the new account
	latestBlockID, err = serviceClient.GetLatestBlockID(ctx)
	s.Require().NoError(err)

	transferTx := buildFlowTransferTx(s.T(), newAccountAddress, "1.0")
	transferTx.
		SetReferenceBlockID(sdk.Identifier(latestBlockID)).
		SetProposalKey(serviceClient.SDKServiceAddress(), 0, serviceClient.GetAndIncrementSeqNumber()).
		SetPayer(serviceClient.SDKServiceAddress()).
		SetComputeLimit(9999)

	err = serviceClient.SignAndSendTransaction(ctx, transferTx)
	s.Require().NoError(err)

	transferTxResult, err := serviceClient.WaitForSealed(ctx, transferTx.ID())
	s.Require().NoError(err)
	s.Require().NoError(transferTxResult.Error)
	s.T().Logf("transfer tx sealed at height %d, tx ID: %s", transferTxResult.BlockHeight, transferTx.ID())

	// Step 4: Wait for the extended indexer to process these blocks
	waitCtx, waitCancel := context.WithTimeout(ctx, 120*time.Second)
	defer waitCancel()

	err = serviceClient.WaitUntilIndexed(waitCtx, transferTxResult.BlockHeight+10)
	s.Require().NoError(err)

	serviceAddr := flow.Address(serviceClient.SDKServiceAddress())
	newAddr := flow.Address(newAccountAddress)
	transferTxID := flow.Identifier(transferTx.ID())

	return []transactionExpectation{
		{
			txID:            createAccountTxID,
			address:         serviceAddr,
			expectedRoles:   []string{"authorizer", "payer", "proposer", "interacted"},
			unexpectedRoles: nil,
		},
		{
			txID:            createAccountTxID,
			address:         newAddr,
			expectedRoles:   []string{"interacted"},
			unexpectedRoles: []string{"authorizer", "payer", "proposer"},
		},
		{
			txID:            transferTxID,
			address:         serviceAddr,
			expectedRoles:   []string{"authorizer", "payer", "proposer", "interacted"},
			unexpectedRoles: nil,
		},
		{
			txID:            transferTxID,
			address:         newAddr,
			expectedRoles:   []string{"interacted"},
			unexpectedRoles: []string{"authorizer", "payer", "proposer"},
		},
	}
}

// verifyAccountTransactionRoles fetches account transactions from the REST API and verifies that
// the given transaction has the expected roles and does not have the unexpected roles.
func (s *ExtendedIndexingSuite) verifyAccountTransactionRoles(
	address string,
	txID string,
	expectedRoles []string,
	unexpectedRoles []string,
) {
	allTxs := s.collectAllPages(address, 50, nil, nil)
	s.T().Logf("account %s has %d transactions via REST API", address, len(allTxs))

	var foundTx *swagger.AccountTransaction
	for i, tx := range allTxs {
		if tx.TransactionId == txID {
			foundTx = &allTxs[i]
			break
		}
	}
	s.Require().NotNil(foundTx, "tx %s not found in account %s REST API response", txID, address)

	for _, role := range expectedRoles {
		s.Contains(foundTx.Roles, role, "account %s should have role %s for tx %s", address, role, txID)
	}
	for _, role := range unexpectedRoles {
		s.NotContains(foundTx.Roles, role, "account %s should not have role %s for tx %s", address, role, txID)
	}
}

// verifyPagination verifies the REST API pagination behavior for the given account. It checks that
// limit=1 returns exactly 1 result with a next_cursor, that the second page contains a different
// transaction, and that paginating through all results yields the same total count as an unpaginated request.
func (s *ExtendedIndexingSuite) verifyPagination(address string) {
	// Paginating through all results should yield the same count as a single unpaginated request
	allUnpaginated := s.fetchAccountTransactions(address, nil)
	allPaginated := s.collectAllPages(address, 1, nil, nil)
	s.Require().Len(allUnpaginated.Transactions, len(allPaginated), "unpaginated and paginated transactions should have the same length")

	for i, unpagedTx := range allUnpaginated.Transactions {
		pagedTx := allPaginated[i]
		s.Equal(unpagedTx, pagedTx, "paged transaction should be the same as the unpaged transaction")
		if i > 0 {
			s.NotEqual(unpagedTx.TransactionId, allUnpaginated.Transactions[i-1].TransactionId, "paged transaction should have a different transaction ID than the previous unpaged transaction")
			s.NotEqual(pagedTx.TransactionId, allPaginated[i-1].TransactionId, "paged transaction should have a different transaction ID than the previous paged transaction")
		}
	}

}

// verifyRoleFiltering verifies that the REST API role filter returns only transactions with the
// requested role and that the filtered set is a subset of the unfiltered set.
func (s *ExtendedIndexingSuite) verifyRoleFiltering(address string) {
	unfilteredResp := s.fetchAccountTransactions(address, nil)

	role := swagger.AUTHORIZER_Role
	authResp := s.fetchAccountTransactions(address, &swagger.AccountsApiGetAccountTransactionsOpts{
		Roles: optional.NewInterface(role),
	})

	expectedCount := 0
	for _, tx := range unfilteredResp.Transactions {
		if slices.Contains(tx.Roles, string(role)) {
			s.Contains(tx.Roles, "authorizer", "expected transaction should have authorizer role")
			expectedCount++
		}
	}
	s.Len(authResp.Transactions, expectedCount, "filtered results should be the same length as the expected results")
}

// fetchAccountTransactions calls the experimental API client to fetch account transactions.
// It retries on errors to account for extended indexer lag.
func (s *ExtendedIndexingSuite) fetchAccountTransactions(
	address string,
	opts *swagger.AccountsApiGetAccountTransactionsOpts,
) *swagger.AccountTransactionsResponse {
	t := s.T()
	ctx := context.Background()

	var result *swagger.AccountTransactionsResponse
	require.Eventually(t, func() bool {
		resp, err := s.apiClient.GetAccountTransactions(ctx, address, opts)
		if err != nil {
			t.Logf("API request failed: %v", err)
			return false
		}
		result = resp
		return true
	}, 30*time.Second, 1*time.Second, "REST API request should succeed for account %s", address)

	return result
}

// collectAllPages paginates through all results for an account and returns all transaction entries.
func (s *ExtendedIndexingSuite) collectAllPages(
	address string,
	pageSize int,
	roles *swagger.Role,
	expand *[]string,
) []swagger.AccountTransaction {
	ctx := context.Background()
	all, err := s.apiClient.GetAllAccountTransactions(ctx, address, pageSize, roles, expand)
	s.Require().NoError(err)
	return all
}

// verifyTransactionDetailsFromAPI verifies that the account transactions API returns populated
// transaction bodies and results, and that these match the data returned by the standard REST API
// endpoints (/v1/transactions/{id} and /v1/transaction_results/{id}).
func (s *ExtendedIndexingSuite) verifyTransactionDetailsFromAPI(expectations []transactionExpectation) {
	// Collect unique transaction IDs from expectations to avoid verifying the same tx twice
	// (a tx can appear in multiple expectations for different addresses).
	verified := make(map[string]bool)

	for _, exp := range expectations {
		txID := exp.txID.String()
		if verified[txID] {
			continue
		}
		verified[txID] = true

		// Fetch the account transactions for this address and find the specific tx
		expand := []string{"transaction", "result"}
		allTxs := s.collectAllPages(exp.address.String(), 50, nil, &expand)
		var acctTx *swagger.AccountTransaction
		for i, tx := range allTxs {
			if tx.TransactionId == txID {
				acctTx = &allTxs[i]
				break
			}
		}
		s.Require().NotNil(acctTx, "tx %s not found in account %s transactions", txID, exp.address.String())

		s.verifyTransactionDetails(*acctTx)
	}
}

// verifyTransactionDetails asserts that the Transaction and Result fields within an AccountTransaction
// are populated and that their key fields match the data from the standard REST API endpoints.
// Field-by-field comparison is used because:
//   - The standard REST API returns raw JSON (map[string]any) while the experimental API returns typed structs.
//   - Event payloads are encoded differently (standard API uses base64 JSON, experimental API uses CCF/CBOR).
func (s *ExtendedIndexingSuite) verifyTransactionDetails(acctTx swagger.AccountTransaction) {
	txID := acctTx.TransactionId

	s.Require().NotNil(acctTx.Transaction, "Transaction body should be populated for tx %s", txID)
	s.Require().NotNil(acctTx.Result, "Transaction result should be populated for tx %s", txID)

	// Fetch from standard REST API
	restTx := s.fetchRESTTransaction(txID)
	restResult := s.fetchRESTTransactionResult(txID)

	// Compare transaction body fields
	s.Equal(restTx["id"], acctTx.Transaction.Id, "transaction ID should match")
	s.Equal(restTx["script"], acctTx.Transaction.Script, "script should match")
	s.Equal(restTx["payer"], acctTx.Transaction.Payer, "payer should match")
	s.Equal(restTx["gas_limit"], acctTx.Transaction.GasLimit, "gas_limit should match")
	s.Equal(restTx["reference_block_id"], acctTx.Transaction.ReferenceBlockId, "reference_block_id should match")

	restAuthorizers, ok := restTx["authorizers"].([]any)
	s.Require().True(ok, "authorizers should be an array")
	s.Require().Equal(len(restAuthorizers), len(acctTx.Transaction.Authorizers), "authorizer count should match")
	for i, a := range restAuthorizers {
		s.Equal(a, acctTx.Transaction.Authorizers[i], "authorizer %d should match", i)
	}

	// Compare result fields
	s.Equal(restResult["block_id"], acctTx.Result.BlockId, "block_id should match")
	s.Equal(restResult["status"], string(*acctTx.Result.Status), "status should match")
	s.Equal(restResult["error_message"], acctTx.Result.ErrorMessage, "error_message should match")
	s.Equal(restResult["collection_id"], acctTx.Result.CollectionId, "collection_id should match")

	// JSON numbers decode to float64 in map[string]any, so convert before comparing.
	restStatusCode, ok := restResult["status_code"].(float64)
	s.Require().True(ok, "status_code should be a number")
	s.Equal(int32(restStatusCode), acctTx.Result.StatusCode, "status_code should match")

	// Compare events by count and type/index (skip payload since encodings differ).
	restEvents, ok := restResult["events"].([]any)
	s.Require().True(ok, "events should be an array")
	s.Require().Equal(len(restEvents), len(acctTx.Result.Events), "event count should match")
	for i, restEvt := range restEvents {
		evtMap, ok := restEvt.(map[string]any)
		s.Require().True(ok, "event should be an object")
		s.Equal(evtMap["type"], acctTx.Result.Events[i].Type_, "event type should match for event %d", i)
		s.Equal(evtMap["event_index"], acctTx.Result.Events[i].EventIndex, "event_index should match for event %d", i)
	}

	s.T().Logf("verified transaction details for tx %s: body and result match standard REST API", txID)
}

// fetchRESTTransaction fetches a transaction from the standard REST API endpoint /v1/transactions/{id}.
func (s *ExtendedIndexingSuite) fetchRESTTransaction(txID string) map[string]any {
	url := fmt.Sprintf("%s/v1/transactions/%s", s.restBaseURL, txID)
	return s.fetchRESTJSON(url, "transaction "+txID)
}

// fetchRESTTransactionResult fetches a transaction result from the standard REST API endpoint
// /v1/transaction_results/{id}.
func (s *ExtendedIndexingSuite) fetchRESTTransactionResult(txID string) map[string]any {
	url := fmt.Sprintf("%s/v1/transaction_results/%s", s.restBaseURL, txID)
	return s.fetchRESTJSON(url, "transaction result "+txID)
}

// fetchRESTJSON performs an HTTP GET to the given URL and returns the decoded JSON body as a map.
// It retries with require.Eventually to handle timing issues.
func (s *ExtendedIndexingSuite) fetchRESTJSON(url string, desc string) map[string]any {
	var result map[string]any
	require.Eventually(s.T(), func() bool {
		resp, err := http.Get(url) //nolint:gosec
		if err != nil {
			s.T().Logf("GET %s failed: %v", desc, err)
			return false
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			s.T().Logf("GET %s returned status %d: %s", desc, resp.StatusCode, string(body))
			return false
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			s.T().Logf("GET %s read body failed: %v", desc, err)
			return false
		}

		if err := json.Unmarshal(body, &result); err != nil {
			s.T().Logf("GET %s JSON decode failed: %v", desc, err)
			return false
		}
		return true
	}, 30*time.Second, 1*time.Second, "REST API request should succeed for %s", desc)

	return result
}

// buildFlowTransferTx constructs a Cadence transaction that transfers Flow tokens to the given address.
func buildFlowTransferTx(t *testing.T, to sdk.Address, amount string) *sdk.Transaction {
	contracts := systemcontracts.SystemContractsForChain(flow.Localnet)
	ftAddr := contracts.FungibleToken.Address.Hex()
	flowTokenAddr := contracts.FlowToken.Address.Hex()

	script := fmt.Sprintf(sendFlowTokensScript, ftAddr, flowTokenAddr)

	amountArg, err := cadence.NewUFix64(amount)
	require.NoError(t, err)
	toArg := cadence.NewAddress(cadence.BytesToAddress(to.Bytes()))

	tx := sdk.NewTransaction().
		SetScript([]byte(strings.TrimSpace(script))).
		AddAuthorizer(sdk.Address(flow.Localnet.Chain().ServiceAddress()))

	err = tx.AddArgument(amountArg)
	require.NoError(t, err)

	err = tx.AddArgument(toArg)
	require.NoError(t, err)

	return tx
}
