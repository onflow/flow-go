package request

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestGetTransactionResult_InvalidParse verifies that parseGetTransactionResult correctly
// returns errors for invalid input combinations and malformed request parameters.
func TestGetTransactionResult_InvalidParse(t *testing.T) {
	validTxID := unittest.IdentifierFixture().String()
	validRequiredExecutorsIds := unittest.IdentifierListFixture(2).Strings()
	invalidID := "invalid-id"

	tests := []struct {
		name                    string
		txID                    string
		blockID                 string
		collectionID            string
		agreeingExecutorsCount  string
		requiredExecutors       []string
		includeExecutorMetadata string
		err                     string
	}{
		{
			"parse with invalid tx ID",
			invalidID,
			"",
			"",
			"2",
			validRequiredExecutorsIds,
			"false",
			"invalid ID format",
		},
		{
			"parse with invalid block ID",
			validTxID,
			invalidID,
			"",
			"2",
			validRequiredExecutorsIds,
			"false",
			"invalid block ID",
		},
		{
			"parse with invalid collection ID",
			validTxID,
			"",
			invalidID,
			"2",
			validRequiredExecutorsIds,
			"false",
			"invalid collection ID",
		},
		{
			"parse with invalid agreeingExecutorCount value",
			validTxID,
			"",
			"",
			"abc",
			validRequiredExecutorsIds,
			"false",
			"invalid agreeingExecutorCount",
		},

		{
			"parse with invalid agreeingExecutorCount number",
			validTxID,
			"",
			"",
			"-1",
			validRequiredExecutorsIds,
			"false",
			"invalid agreeingExecutorCount",
		},
		{
			"parse with invalid requiredExecutorsIds",
			validTxID,
			"",
			"",
			"2",
			[]string{"not-a-valid-id"},
			"false",
			"invalid ID format",
		},
		{
			"parse with invalid includeExecutorMetadata",
			validTxID,
			"",
			"",
			"2",
			validRequiredExecutorsIds,
			"not-bool",
			"invalid includeExecutorMetadata",
		},
	}

	for i, test := range tests {
		url := transactionResultURL(
			test.txID,
			test.blockID,
			test.collectionID,
			test.agreeingExecutorsCount,
			test.requiredExecutors,
			test.includeExecutorMetadata,
		)
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		req = mux.SetURLVars(req, map[string]string{
			idQuery: test.txID,
		})
		r := common.Decorate(req, flow.Testnet.Chain())
		getTransactionResult, err := parseGetTransactionResult(
			r,
			test.agreeingExecutorsCount,
			test.requiredExecutors,
			test.includeExecutorMetadata,
		)

		require.Nil(t, getTransactionResult)
		require.ErrorContains(t, err, test.err, fmt.Sprintf("test #%d failed", i))
	}
}

// TestGetTransactionResult_ValidParse verifies that parseGetTransactionResult successfully
// parses valid GetTransactionResult requests and populates the request fields as expected.
func TestGetTransactionResult_ValidParse(t *testing.T) {
	txID := unittest.IdentifierFixture()
	blockID := unittest.IdentifierFixture()
	collectionID := unittest.IdentifierFixture()
	agreeingExecutorsCount := uint64(2)

	validAgreeingExecutorsIds := unittest.IdentifierListFixture(2)
	agreeingExecutorsCountStr := fmt.Sprintf("%d", agreeingExecutorsCount)
	validAgreeingExecutorsIdsStr := validAgreeingExecutorsIds.Strings()
	includeExecutorMetadataStr := "true"
	url := transactionResultURL(txID.String(), blockID.String(), collectionID.String(), agreeingExecutorsCountStr, validAgreeingExecutorsIdsStr, includeExecutorMetadataStr)
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req = mux.SetURLVars(req, map[string]string{
		idQuery: txID.String(),
	})
	r := common.Decorate(req, flow.Testnet.Chain())

	getTransactionResult, err := parseGetTransactionResult(
		r,
		agreeingExecutorsCountStr,
		validAgreeingExecutorsIdsStr,
		includeExecutorMetadataStr,
	)
	require.NoError(t, err)
	require.NotNil(t, getTransactionResult)
	require.Equal(t, getTransactionResult.GetByIDRequest.ID, txID)
	require.Equal(t, getTransactionResult.TransactionOptionals.BlockID, blockID)
	require.Equal(t, getTransactionResult.TransactionOptionals.CollectionID, collectionID)
	require.Equal(t, getTransactionResult.ExecutionState.AgreeingExecutorsCount, agreeingExecutorsCount)
	require.EqualValues(t, getTransactionResult.ExecutionState.RequiredExecutorIDs, validAgreeingExecutorsIds)
	require.True(t, getTransactionResult.ExecutionState.IncludeExecutorMetadata)
}

func transactionResultURL(
	txID string,
	blockID string,
	collectionID string,
	agreeingExecutorsCount string,
	requiredExecutors []string,
	includeExecutorMetadata string,
) string {
	u, _ := url.Parse(fmt.Sprintf("/v1/transaction_results/%s", txID))
	q := u.Query()
	if blockID != "" {
		q.Add("block_id", blockID)
	}

	if collectionID != "" {
		q.Add("collection_id", collectionID)
	}

	if len(agreeingExecutorsCount) > 0 {
		q.Add(agreeingExecutorCountQuery, agreeingExecutorsCount)
	}

	if len(requiredExecutors) > 0 {
		q.Add(requiredExecutorIdsQuery, strings.Join(requiredExecutors, ","))
	}

	if len(includeExecutorMetadata) > 0 {
		q.Add(includeExecutorMetadataQuery, includeExecutorMetadata)
	}

	u.RawQuery = q.Encode()
	return u.String()
}
