package cohort3

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	sdk "github.com/onflow/flow-go-sdk"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/integration/testnet"
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
//     - GET /experimental/v1/contracts/account/{address} scopes to the service account
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

	// ---- Step 4: Verify GET /experimental/v1/contracts/{identifier} ----
	s.verifyContractLatestDeployment(contractID, updateResult.BlockHeight)

	// ---- Step 5: Verify GET /experimental/v1/contracts/{identifier}/deployments ----
	s.verifyContractDeploymentHistory(contractID, deployResult.BlockHeight, updateResult.BlockHeight)

	// ---- Step 6: Verify GET /experimental/v1/contracts lists the contract ----
	s.verifyContractInList(contractID, updateResult.BlockHeight)

	// ---- Step 7: Verify GET /experimental/v1/contracts/account/{address} scopes correctly ----
	s.verifyContractsByAddress(serviceAddr.String(), contractID)

	// ---- Step 8: Verify pagination for deployments ----
	s.verifyContractDeploymentPagination(contractID)
}

// verifyContractLatestDeployment polls GET /experimental/v1/contracts/{identifier} until it
// returns the most recent deployment (at expectedHeight), then asserts all key fields.
func (s *ExtendedIndexingSuite) verifyContractLatestDeployment(contractID string, expectedHeight uint64) {
	url := fmt.Sprintf("%s/experimental/v1/contracts/%s", s.restBaseURL, contractID)

	var deployment map[string]any
	require.Eventually(s.T(), func() bool {
		d := s.fetchContractJSON(url)
		if d == nil {
			return false
		}
		blockHeight, _ := d["block_height"].(float64)
		if uint64(blockHeight) != expectedHeight {
			s.T().Logf("waiting for latest deployment at height %d, got %.0f", expectedHeight, blockHeight)
			return false
		}
		deployment = d
		return true
	}, 60*time.Second, 2*time.Second, "latest deployment should appear at height %d", expectedHeight)

	s.Equal(contractID, deployment["identifier"], "identifier should match")
	s.NotEmpty(deployment["transaction_id"], "transaction_id should be set")
	s.NotEmpty(deployment["code_hash"], "code_hash should be set")
	s.NotEmpty(deployment["code"], "code should be populated")
	s.T().Logf("verified latest deployment for %s at height %.0f", contractID, deployment["block_height"])
}

// verifyContractDeploymentHistory polls GET /experimental/v1/contracts/{identifier}/deployments
// until two deployments are present, then asserts correct ordering (newest first).
func (s *ExtendedIndexingSuite) verifyContractDeploymentHistory(contractID string, deployHeight, updateHeight uint64) {
	url := fmt.Sprintf("%s/experimental/v1/contracts/%s/deployments", s.restBaseURL, contractID)

	var contracts []map[string]any
	require.Eventually(s.T(), func() bool {
		body := s.fetchContractJSON(url)
		if body == nil {
			return false
		}
		raw, _ := body["contracts"].([]any)
		cs := toMapSlice(raw)
		if len(cs) < 2 {
			s.T().Logf("waiting for 2 deployments, got %d", len(cs))
			return false
		}
		contracts = cs
		return true
	}, 60*time.Second, 2*time.Second, "deployment history should contain 2 entries")

	s.Require().Len(contracts, 2, "should have exactly 2 deployments")

	// Deployments are ordered newest-first.
	h0, _ := contracts[0]["block_height"].(float64)
	h1, _ := contracts[1]["block_height"].(float64)
	s.Equal(updateHeight, uint64(h0), "first entry should be the update deployment")
	s.Equal(deployHeight, uint64(h1), "second entry should be the initial deployment")

	s.T().Logf("verified deployment history for %s: heights %.0f, %.0f", contractID, h0, h1)
}

// verifyContractInList polls GET /experimental/v1/contracts (paginating all results) until the
// expected contract appears with its latest deployment height.
func (s *ExtendedIndexingSuite) verifyContractInList(contractID string, expectedHeight uint64) {
	require.Eventually(s.T(), func() bool {
		all := s.fetchAllContractPages(20)
		for _, c := range all {
			if id, _ := c["identifier"].(string); id == contractID {
				h, _ := c["block_height"].(float64)
				if uint64(h) == expectedHeight {
					return true
				}
				s.T().Logf("waiting for %s at height %d in /contracts list, got %.0f", contractID, expectedHeight, h)
			}
		}
		s.T().Logf("contract %s not yet in /contracts list (%d total)", contractID, len(all))
		return false
	}, 60*time.Second, 2*time.Second, "contract %s should appear in /contracts list at height %d", contractID, expectedHeight)

	s.T().Logf("verified %s appears in /contracts list", contractID)
}

// verifyContractsByAddress polls GET /experimental/v1/contracts/account/{address} and verifies
// the expected contract is present at its latest deployment.
func (s *ExtendedIndexingSuite) verifyContractsByAddress(address string, contractID string) {
	require.Eventually(s.T(), func() bool {
		all := s.fetchAllContractsByAddressPages(address, 20)
		for _, c := range all {
			if id, _ := c["identifier"].(string); id == contractID {
				return true
			}
		}
		s.T().Logf("contract %s not yet in /contracts/account/%s list (%d total)", contractID, address, len(all))
		return false
	}, 30*time.Second, 1*time.Second, "contract %s should appear under address %s", contractID, address)

	s.T().Logf("verified %s appears under address %s", contractID, address)
}

// verifyContractDeploymentPagination verifies that paginating through
// GET /experimental/v1/contracts/{identifier}/deployments with limit=1 returns the same entries
// as a single large-page request.
func (s *ExtendedIndexingSuite) verifyContractDeploymentPagination(contractID string) {
	allAtOnce := s.fetchAllContractDeploymentPages(contractID, 100)

	// Collect all pages one at a time.
	allPaged := s.fetchAllContractDeploymentPages(contractID, 1)

	s.Require().Equal(len(allAtOnce), len(allPaged),
		"paginated deployments should equal bulk-fetched deployments")

	for i := range allAtOnce {
		h0, _ := allAtOnce[i]["block_height"].(float64)
		hp, _ := allPaged[i]["block_height"].(float64)
		s.Equal(h0, hp, "deployment at index %d should have matching block_height", i)
	}

	s.T().Logf("pagination verified for %s: %d deployments", contractID, len(allAtOnce))
}

// ===== HTTP helpers =====

// fetchAllContractPages paginates GET /experimental/v1/contracts and returns all contract entries.
func (s *ExtendedIndexingSuite) fetchAllContractPages(pageSize int) []map[string]any {
	firstURL := fmt.Sprintf("%s/experimental/v1/contracts?limit=%d", s.restBaseURL, pageSize)
	return s.collectContractPages(firstURL, "contracts")
}

// fetchAllContractsByAddressPages paginates GET /experimental/v1/contracts/account/{address}
// and returns all contract entries.
func (s *ExtendedIndexingSuite) fetchAllContractsByAddressPages(address string, pageSize int) []map[string]any {
	firstURL := fmt.Sprintf("%s/experimental/v1/contracts/account/%s?limit=%d", s.restBaseURL, address, pageSize)
	return s.collectContractPages(firstURL, "contracts")
}

// fetchAllContractDeploymentPages paginates GET /experimental/v1/contracts/{id}/deployments
// and returns all deployment entries.
func (s *ExtendedIndexingSuite) fetchAllContractDeploymentPages(contractID string, pageSize int) []map[string]any {
	firstURL := fmt.Sprintf("%s/experimental/v1/contracts/%s/deployments?limit=%d", s.restBaseURL, contractID, pageSize)
	return s.collectContractPages(firstURL, "contracts")
}

// collectContractPages follows next_cursor links and collects all entries under the given JSON key.
// firstURL must already include the limit query parameter.
func (s *ExtendedIndexingSuite) collectContractPages(firstURL string, key string) []map[string]any {
	var all []map[string]any
	url := firstURL
	for {
		body := s.fetchContractJSONWithRetry(url)
		if body == nil {
			break
		}
		raw, _ := body[key].([]any)
		all = append(all, toMapSlice(raw)...)

		nextCursor, _ := body["next_cursor"].(string)
		if nextCursor == "" {
			break
		}

		// Determine the base path to construct the next page URL.
		// The cursor is appended as a query parameter to the current page's base URL (without cursor).
		url = fmt.Sprintf("%s&cursor=%s", firstURL, nextCursor)
	}
	return all
}

// fetchContractJSONWithRetry fetches JSON from the given URL with require.Eventually retry logic.
func (s *ExtendedIndexingSuite) fetchContractJSONWithRetry(url string) map[string]any {
	var result map[string]any
	require.Eventually(s.T(), func() bool {
		r := s.fetchContractJSON(url)
		if r == nil {
			return false
		}
		result = r
		return true
	}, 30*time.Second, 1*time.Second, "REST GET %s should succeed", url)
	return result
}

// fetchContractJSON performs a single HTTP GET and returns the decoded JSON body, or nil on failure.
// A 400 Bad Request is treated as retryable because codes.FailedPrecondition (index not yet
// bootstrapped) maps to HTTP 400 in this REST framework.
func (s *ExtendedIndexingSuite) fetchContractJSON(url string) map[string]any {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		s.T().Logf("GET %s returned status %d: %s", url, resp.StatusCode, string(body))
		return nil
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		s.T().Logf("GET %s JSON decode failed: %v", url, err)
		return nil
	}
	return result
}
