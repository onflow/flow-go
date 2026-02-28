package cohort3

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	sdk "github.com/onflow/flow-go-sdk"
	"github.com/stretchr/testify/require"

	swagger "github.com/onflow/flow/openapi/experimental/go-client-generated"

	"github.com/onflow/flow-go/integration/testnet"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/dsl"
)

// testContractName is the name used for the contract deployed during contract indexing tests.
const testContractName = "IndexingTestContract"

// contractV1 is the initial version of the test contract deployed to the service account.
var contractV1 = dsl.Contract{
	Name: testContractName,
	Members: []dsl.CadenceCode{
		dsl.Code(`
			access(all) let version: String
			init() { self.version = "1.0" }
		`),
	},
}

// contractV2 is the updated version of the test contract, used to exercise the update path.
var contractV2 = dsl.Contract{
	Name: testContractName,
	Members: []dsl.CadenceCode{
		dsl.Code(`
			access(all) let version: String
			init() { self.version = "2.0" }
		`),
	},
}

// TestContractLifecycle verifies the full contract deployment lifecycle through the extended indexer:
//  1. Deploy a contract (AccountContractAdded → deployment indexed at the deploy block)
//  2. Update the contract (AccountContractUpdated → second deployment indexed)
//  3. Verify all REST endpoints:
//     - GET /experimental/v1/contracts/{identifier} returns the latest deployment
//     - GET /experimental/v1/contracts/{identifier}/deployments returns both in newest-first order
//     - GET /experimental/v1/contracts lists the contract
//     - GET /experimental/v1/accounts/{address}/contracts scopes to the service account
//     - Pagination via next_cursor works for both /contracts and /deployments
func (s *ExtendedIndexingSuite) TestContractLifecycle() {
	t := s.T()
	ctx := context.Background()

	t.Logf("%v ================> START TESTING %v", time.Now().UTC(), t.Name())
	defer t.Logf("%v ================> FINISH TESTING %v", time.Now().UTC(), t.Name())

	accessClient, err := s.net.ContainerByName(testnet.PrimaryAN).TestnetClient()
	s.Require().NoError(err)

	// ---- Step 1: Deploy contractV1 ----
	refBlockID, err := accessClient.GetLatestBlockID(ctx)
	s.Require().NoError(err)

	deployTx, err := accessClient.DeployContract(ctx, sdk.Identifier(refBlockID), contractV1)
	s.Require().NoError(err)

	deployResult, err := accessClient.WaitForSealed(ctx, deployTx.ID())
	s.Require().NoError(err)
	s.Require().NoError(deployResult.Error, "contract deploy transaction should succeed")
	t.Logf("contract v1 deployed at block %d, tx %s", deployResult.BlockHeight, deployTx.ID())

	// ---- Step 2: Update to contractV2 ----
	refBlockID, err = accessClient.GetLatestBlockID(ctx)
	s.Require().NoError(err)

	updateTx, err := accessClient.UpdateContract(ctx, sdk.Identifier(refBlockID), contractV2)
	s.Require().NoError(err)

	updateResult, err := accessClient.WaitForSealed(ctx, updateTx.ID())
	s.Require().NoError(err)
	s.Require().NoError(updateResult.Error, "contract update transaction should succeed")
	t.Logf("contract v2 deployed at block %d, tx %s", updateResult.BlockHeight, updateTx.ID())

	// ---- Step 3: Wait for the extended indexer to process both blocks ----
	waitCtx, waitCancel := context.WithTimeout(ctx, 120*time.Second)
	defer waitCancel()

	err = accessClient.WaitUntilIndexed(waitCtx, updateResult.BlockHeight+5)
	s.Require().NoError(err, "extended indexer did not catch up in time")
	t.Log("extended indexer caught up")

	// Derive identifiers for assertions.
	serviceAddr := flow.Address(accessClient.SDKServiceAddress())
	contractID := fmt.Sprintf("A.%s.%s", serviceAddr.Hex(), testContractName)
	t.Logf("contract identifier: %s", contractID)

	deploy1Code := []byte(contractV1.ToCadence())
	deploy1CodeHash := accessmodel.CadenceCodeHash(deploy1Code)
	deploy1 := accessmodel.ContractDeployment{
		ContractID:    contractID,
		Address:       serviceAddr,
		BlockHeight:   deployResult.BlockHeight,
		TransactionID: flow.Identifier(deployTx.ID()),
		Code:          deploy1Code,
		CodeHash:      deploy1CodeHash,
	}

	deploy2Code := []byte(contractV2.ToCadence())
	deploy2CodeHash := accessmodel.CadenceCodeHash(deploy2Code)
	deploy2 := accessmodel.ContractDeployment{
		ContractID:    contractID,
		Address:       serviceAddr,
		BlockHeight:   updateResult.BlockHeight,
		TransactionID: flow.Identifier(updateTx.ID()),
		Code:          deploy2Code,
		CodeHash:      deploy2CodeHash[:],
	}

	// ---- Step 4: Verify GET /experimental/v1/contracts/{identifier} ----
	s.verifyContractLatestDeployment(contractID, deploy2)

	// ---- Step 5: Verify GET /experimental/v1/contracts/{identifier}/deployments ----
	// The API returns deployments newest-first (descending block height).
	s.verifyContractDeploymentHistory(contractID, []accessmodel.ContractDeployment{deploy2, deploy1})

	// ---- Step 6: Verify GET /experimental/v1/contracts lists the contract ----
	s.verifyContractInList(contractID, deploy2)

	// ---- Step 7: Verify GET /experimental/v1/accounts/{address}/contracts scopes correctly ----
	s.verifyContractsByAddress(serviceAddr.Hex(), deploy2)

	// ---- Step 8: Verify pagination for deployments ----
	s.verifyContractDeploymentPagination(contractID)
}

// verifyContractLatestDeployment polls GET /experimental/v1/contracts/{identifier} until it
// returns the most recent deployment, then asserts all key fields.
func (s *ExtendedIndexingSuite) verifyContractLatestDeployment(contractID string, expected accessmodel.ContractDeployment) {
	ctx := context.Background()

	var d *swagger.ContractDeployment
	require.Eventually(s.T(), func() bool {
		resp, err := s.apiClient.GetContractByIdentifier(ctx, contractID)
		if err != nil {
			s.T().Logf("GET contract %s failed: %v", contractID, err)
			return false
		}
		d = resp
		return true
	}, 30*time.Second, 1*time.Second, "GET contract %s should succeed", contractID)

	s.Equal(contractID, d.ContractId, "contract_id should match")
	s.Equal(strconv.FormatUint(expected.BlockHeight, 10), d.BlockHeight, "block_height should match")
	s.Equal(expected.TransactionID.String(), d.TransactionId, "transaction_id should match")
	s.Equal(base64.StdEncoding.EncodeToString(expected.Code), d.Code, "code should match")
	s.Equal(hex.EncodeToString(expected.CodeHash), d.CodeHash, "code_hash should match")

	s.T().Logf("verified latest deployment for %s at height %s", contractID, d.BlockHeight)
}

// verifyContractDeploymentHistory polls GET /experimental/v1/contracts/{identifier}/deployments
// until two deployments are present, then asserts correct ordering (newest first).
func (s *ExtendedIndexingSuite) verifyContractDeploymentHistory(contractID string, expected []accessmodel.ContractDeployment) {
	ctx := context.Background()

	var deployments []swagger.ContractDeployment
	require.Eventually(s.T(), func() bool {
		resp, err := s.apiClient.GetContractDeployments(ctx, contractID, nil)
		if err != nil {
			s.T().Logf("GET contract deployments %s failed: %v", contractID, err)
			return false
		}
		deployments = resp.Deployments
		return true
	}, 30*time.Second, 1*time.Second, "GET contract deployments %s should succeed", contractID)

	s.Require().Len(deployments, 2, "should have exactly 2 deployments")

	for i, exp := range expected {
		d := deployments[i]
		s.Equal(exp.ContractID, d.ContractId, "deployment at index %d should have matching contract_id", i)
		s.Equal(strconv.FormatUint(exp.BlockHeight, 10), d.BlockHeight, "deployment at index %d should have matching block_height", i)
		s.Equal(exp.TransactionID.String(), d.TransactionId, "deployment at index %d should have matching transaction_id", i)
		s.Equal(base64.StdEncoding.EncodeToString(exp.Code), d.Code, "deployment at index %d should have matching code", i)
		s.Equal(hex.EncodeToString(exp.CodeHash), d.CodeHash, "deployment at index %d should have matching code_hash", i)
	}

	s.T().Logf("verified deployment history for %s: %d deployments", contractID, len(expected))
}

// verifyContractInList paginates GET /experimental/v1/contracts until the expected contract
// appears with its latest deployment, then asserts all key fields.
func (s *ExtendedIndexingSuite) verifyContractInList(contractID string, expected accessmodel.ContractDeployment) {
	ctx := context.Background()

	var all []swagger.ContractDeployment
	require.Eventually(s.T(), func() bool {
		contracts, err := s.apiClient.GetAllContracts(ctx, 20)
		if err != nil {
			s.T().Logf("GET /contracts failed: %v", err)
			return false
		}
		all = contracts
		return true
	}, 30*time.Second, 1*time.Second, "GET /contracts should succeed")

	for _, d := range all {
		if d.ContractId == contractID {
			s.Equal(expected.ContractID, d.ContractId, "contract_id should match")
			s.Equal(strconv.FormatUint(expected.BlockHeight, 10), d.BlockHeight, "block_height should match")
			s.Equal(expected.TransactionID.String(), d.TransactionId, "transaction_id should match")
			s.Equal(base64.StdEncoding.EncodeToString(expected.Code), d.Code, "code should match")
			s.Equal(hex.EncodeToString(expected.CodeHash), d.CodeHash, "code_hash should match")
			s.T().Logf("verified %s appears in /contracts list at height %s", contractID, d.BlockHeight)
			return
		}
	}
	s.Require().Fail("contract should appear in /contracts list", "contract %s not found in /contracts list", contractID)
}

// verifyContractsByAddress paginates GET /experimental/v1/accounts/{address}/contracts and
// verifies the expected contract is present at its latest deployment.
func (s *ExtendedIndexingSuite) verifyContractsByAddress(address string, expected accessmodel.ContractDeployment) {
	ctx := context.Background()

	var all []swagger.ContractDeployment
	require.Eventually(s.T(), func() bool {
		contracts, err := s.apiClient.GetAllContractsByAccount(ctx, address, 20)
		if err != nil {
			s.T().Logf("GET /accounts/%s/contracts failed: %v", address, err)
			return false
		}
		all = contracts
		return true
	}, 30*time.Second, 1*time.Second, "GET /accounts/%s/contracts should succeed", address)

	for _, d := range all {
		if d.ContractId == expected.ContractID {
			s.Equal(expected.ContractID, d.ContractId, "contract_id should match")
			s.Equal(strconv.FormatUint(expected.BlockHeight, 10), d.BlockHeight, "block_height should match")
			s.Equal(expected.TransactionID.String(), d.TransactionId, "transaction_id should match")
			s.Equal(base64.StdEncoding.EncodeToString(expected.Code), d.Code, "code should match")
			s.Equal(hex.EncodeToString(expected.CodeHash), d.CodeHash, "code_hash should match")
			s.T().Logf("verified %s appears under address %s", expected.ContractID, address)
			return
		}
	}
	s.Require().Fail("contract should appear in /accounts/%s/contracts list",
		"contract %s not found in /accounts/%s/contracts list", address, expected.ContractID, address)
}

// verifyContractDeploymentPagination verifies that paginating through
// GET /experimental/v1/contracts/{identifier}/deployments with limit=1 returns the same entries
// as a single large-page request.
func (s *ExtendedIndexingSuite) verifyContractDeploymentPagination(contractID string) {
	ctx := context.Background()

	allAtOnce, err := s.apiClient.GetAllContractDeployments(ctx, contractID, 100)
	s.Require().NoError(err, "bulk fetch of contract deployments should succeed")

	allPaged, err := s.apiClient.GetAllContractDeployments(ctx, contractID, 1)
	s.Require().NoError(err, "paginated fetch of contract deployments should succeed")

	s.Require().Equal(len(allAtOnce), len(allPaged),
		"paginated deployments should equal bulk-fetched deployments")

	for i := range allAtOnce {
		s.Equal(allAtOnce[i].BlockHeight, allPaged[i].BlockHeight,
			"deployment at index %d should have matching block_height", i)
	}

	s.T().Logf("pagination verified for %s: %d deployments", contractID, len(allAtOnce))
}
