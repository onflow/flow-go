package cohort3

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
)

// TestScheduledTransactionLifecycle tests the full lifecycle:
// 1. Schedule a transaction (Scheduled event → status "scheduled")
// 2. Wait for execution (Executed event → status "executed")
// 3. Schedule and cancel another (Canceled event → status "cancelled")
// 4. Verify all data via the REST endpoints and pagination.
func (s *ExtendedIndexingSuite) TestScheduledTransactionLifecycle() {
	accessClient, err := s.net.ContainerByName(testnet.PrimaryAN).TestnetClient()
	s.Require().NoError(err)

	sc := systemcontracts.SystemContractsForChain(s.net.Root().HeaderBody.ChainID)

	// Deploy the test handler contract.
	deployTxID, err := lib.DeployScheduledTransactionsTestContract(accessClient, sc)
	s.Require().NoError(err, "could not deploy test handler contract")

	_, err = accessClient.WaitForSealed(context.Background(), deployTxID)
	s.Require().NoError(err)
	s.T().Log("test handler contract deployed")

	// ---- Schedule tx1 with a near-future timestamp so it executes quickly ----
	nearFutureTimestamp := time.Now().Unix() + 5
	scheduledID1, err := lib.ScheduleTransactionAtTimestamp(nearFutureTimestamp, accessClient, sc)
	s.Require().NoError(err)
	s.Require().NotZero(scheduledID1)
	s.T().Logf("scheduled tx1 with scheduler ID: %d", scheduledID1)

	// ---- Schedule tx2 far in the future, then cancel it ----
	futureTimestamp := time.Now().Unix() + 3600
	scheduledID2, err := lib.ScheduleTransactionAtTimestamp(futureTimestamp, accessClient, sc)
	s.Require().NoError(err)
	s.Require().NotZero(scheduledID2)
	s.T().Logf("scheduled tx2 with scheduler ID: %d", scheduledID2)

	cancelledID, err := lib.CancelTransactionByID(scheduledID2, accessClient, sc)
	s.Require().NoError(err)
	s.T().Logf("cancelled tx2 (scheduler ID: %d, cancel event ID: %d)", scheduledID2, cancelledID)

	// ---- Wait for the extended indexer to process enough blocks ----
	latestHeader, err := accessClient.GetLatestFinalizedBlockHeader(context.Background())
	s.Require().NoError(err)

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer waitCancel()
	err = accessClient.WaitUntilIndexed(waitCtx, uint64(latestHeader.Height)+5)
	s.Require().NoError(err, "extended indexer did not catch up in time")
	s.T().Log("extended indexer caught up")

	// ---- Verify tx1 is executed ----
	s.verifyScheduledTxStatus(scheduledID1, "executed")

	// ---- Verify tx2 is cancelled ----
	s.verifyScheduledTxStatus(scheduledID2, "cancelled")

	// ---- Verify the /scheduled endpoint lists both ----
	allTxs := s.fetchAllScheduledTxs(20)
	s.T().Logf("found %d scheduled transactions in /scheduled", len(allTxs))

	var foundID1, foundID2 bool
	for _, tx := range allTxs {
		idStr := fmt.Sprintf("%v", tx["id"])
		if idStr == fmt.Sprintf("%d", scheduledID1) {
			foundID1 = true
			s.Equal("executed", tx["status"], "tx1 should be executed")
		}
		if idStr == fmt.Sprintf("%d", scheduledID2) {
			foundID2 = true
			s.Equal("cancelled", tx["status"], "tx2 should be cancelled")
		}
	}
	s.True(foundID1, "tx1 (executed) should appear in /scheduled")
	s.True(foundID2, "tx2 (cancelled) should appear in /scheduled")

	// ---- Verify /accounts/{address}/scheduled scopes to owner ----
	ownerAddr := flow.Address(accessClient.SDKServiceAddress()).String()
	addrTxs := s.fetchAllScheduledTxsByAddress(ownerAddr, 20)
	s.T().Logf("found %d scheduled transactions in /accounts/{address}/scheduled", len(addrTxs))

	var addrFoundID1, addrFoundID2 bool
	for _, tx := range addrTxs {
		idStr := fmt.Sprintf("%v", tx["id"])
		if idStr == fmt.Sprintf("%d", scheduledID1) {
			addrFoundID1 = true
		}
		if idStr == fmt.Sprintf("%d", scheduledID2) {
			addrFoundID2 = true
		}
	}
	s.True(addrFoundID1, "tx1 should appear in /accounts/{address}/scheduled")
	s.True(addrFoundID2, "tx2 should appear in /accounts/{address}/scheduled")

	// ---- Verify pagination works via /scheduled with limit=1 ----
	s.verifyScheduledTxPagination()

	// ---- Verify status filter ----
	executedTxs := s.fetchScheduledTxsWithFilter("status=executed")
	for _, tx := range executedTxs {
		s.Equal("executed", tx["status"], "status filter should only return executed txs")
	}

	cancelledTxs := s.fetchScheduledTxsWithFilter("status=cancelled")
	for _, tx := range cancelledTxs {
		s.Equal("cancelled", tx["status"], "status filter should only return cancelled txs")
	}
}

// verifyScheduledTxStatus polls GET /experimental/v1/scheduled/transaction/{id} until the
// expected status is returned.
func (s *ExtendedIndexingSuite) verifyScheduledTxStatus(id uint64, expectedStatus string) {
	url := fmt.Sprintf("%s/experimental/v1/scheduled/transaction/%d", s.restBaseURL, id)
	require.Eventually(s.T(), func() bool {
		tx := s.fetchScheduledTxJSON(url)
		if tx == nil {
			return false
		}
		actual, _ := tx["status"].(string)
		if actual != expectedStatus {
			s.T().Logf("waiting for tx %d status %q, got %q", id, expectedStatus, actual)
			return false
		}
		return true
	}, 60*time.Second, 2*time.Second, "tx %d did not reach status %q", id, expectedStatus)
}

// fetchAllScheduledTxs paginates through GET /experimental/v1/scheduled and returns all results.
func (s *ExtendedIndexingSuite) fetchAllScheduledTxs(pageSize int) []map[string]any {
	return s.collectScheduledPages(
		fmt.Sprintf("%s/experimental/v1/scheduled?limit=%d", s.restBaseURL, pageSize),
		pageSize,
	)
}

// fetchAllScheduledTxsByAddress paginates through GET /experimental/v1/accounts/{address}/scheduled.
func (s *ExtendedIndexingSuite) fetchAllScheduledTxsByAddress(address string, pageSize int) []map[string]any {
	return s.collectScheduledPages(
		fmt.Sprintf("%s/experimental/v1/accounts/%s/scheduled?limit=%d", s.restBaseURL, address, pageSize),
		pageSize,
	)
}

// fetchScheduledTxsWithFilter fetches /experimental/v1/scheduled with the given query string filter.
func (s *ExtendedIndexingSuite) fetchScheduledTxsWithFilter(filter string) []map[string]any {
	url := fmt.Sprintf("%s/experimental/v1/scheduled?limit=100&%s", s.restBaseURL, filter)
	body := s.fetchScheduledTxJSONBody(url)
	if body == nil {
		return nil
	}
	txs, _ := body["scheduled_transactions"].([]any)
	return toMapSlice(txs)
}

// verifyScheduledTxPagination verifies that paginating through results one at a time yields the
// same total as fetching all at once.
func (s *ExtendedIndexingSuite) verifyScheduledTxPagination() {
	allAtOnce := s.fetchAllScheduledTxs(100)
	allPaged := s.collectScheduledPages(
		fmt.Sprintf("%s/experimental/v1/scheduled?limit=1", s.restBaseURL),
		1,
	)

	s.Require().Equal(len(allAtOnce), len(allPaged),
		"paginated results should equal unpaginated results")

	for i := range allAtOnce {
		s.Equal(allAtOnce[i]["id"], allPaged[i]["id"],
			"tx at index %d should have the same ID", i)
	}
}

// collectScheduledPages follows next_cursor links to collect all transactions across all pages.
func (s *ExtendedIndexingSuite) collectScheduledPages(firstURL string, pageSize int) []map[string]any {
	var all []map[string]any
	url := firstURL
	for {
		body := s.fetchScheduledTxJSONBody(url)
		if body == nil {
			break
		}
		txs, _ := body["scheduled_transactions"].([]any)
		all = append(all, toMapSlice(txs)...)

		nextCursor, _ := body["next_cursor"].(string)
		if nextCursor == "" {
			break
		}
		url = fmt.Sprintf("%s/experimental/v1/scheduled?limit=%d&cursor=%s",
			s.restBaseURL, pageSize, nextCursor)
	}
	return all
}

// fetchScheduledTxJSON fetches JSON from the given URL. Returns nil on non-200 or error.
func (s *ExtendedIndexingSuite) fetchScheduledTxJSON(url string) map[string]any {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil
	}
	return result
}

// fetchScheduledTxJSONBody fetches JSON from the given URL with require.Eventually retry logic.
func (s *ExtendedIndexingSuite) fetchScheduledTxJSONBody(url string) map[string]any {
	var result map[string]any
	require.Eventually(s.T(), func() bool {
		r := s.fetchScheduledTxJSON(url)
		if r == nil {
			return false
		}
		result = r
		return true
	}, 30*time.Second, 1*time.Second, "REST GET %s should succeed", url)
	return result
}

// toMapSlice converts a []any (from JSON unmarshaling) to []map[string]any.
func toMapSlice(in []any) []map[string]any {
	out := make([]map[string]any, 0, len(in))
	for _, item := range in {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}
