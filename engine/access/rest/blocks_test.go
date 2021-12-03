package rest

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	mocks "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/rest/middleware"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func blockURL(ids []string, start uint64, end uint64, expandPayload bool, expandExecResult bool, heights ...string) string {
	u, _ := url.Parse("/v1/blocks")
	q := u.Query()

	if len(ids) > 0 {
		u, _ = url.Parse(u.String() + "/" + strings.Join(ids, ","))
	}

	if start > 0 {
		q.Add(startHeightQueryParam, fmt.Sprintf("%d", start))
		q.Add(endHeightQueryParam, fmt.Sprintf("%d", end))
	}

	if len(heights) > 0 {
		heightsStr := strings.Join(heights, ",")
		q.Add(heightQueryParam, heightsStr)
	}

	var expands []string
	if expandPayload {
		expands = append(expands, ExpandableFieldPayload)
	}
	if expandExecResult {
		expands = append(expands, ExpandableExecutionResult)
	}
	if len(expands) > 0 {
		expandsStr := strings.Join(expands, ",")
		q.Add(middleware.ExpandQueryParam, expandsStr)
	}

	u.RawQuery = q.Encode()
	return u.String()
}

func TestGetBlocks(t *testing.T) {
	backend := &mock.API{}

	t.Run("get single block by ID", func(t *testing.T) {
		block := unittest.BlockFixture()
		executionResult := unittest.ExecutionResultFixture()

		backend.Mock.On("GetBlockByID", mocks.Anything, block.ID()).Return(&block, nil)
		backend.Mock.On("GetExecutionResultForBlockID", mocks.Anything, block.ID()).Return(executionResult, nil)

		req, err := http.NewRequest("GET", blockURL([]string{block.ID().String()}, 0, 0, true, true), nil)
		require.NoError(t, err)

		rr := executeRequest(req, backend)

		expectedResp := expectedBlockResponsesExpanded([]*flow.Block{&block}, []*flow.ExecutionResult{executionResult})
		assert.Equal(t, http.StatusOK, rr.Code)
		actualResp := rr.Body.String()
		fmt.Println(actualResp)
		assert.JSONEq(t, expectedResp, actualResp)
	})

	t.Run("get multiple blocks by ID", func(t *testing.T) {

		blocks := unittest.BlockFixtures(2)
		executionResults := unittest.ExecutionResultFixtures(2)

		blockIDs := make([]string, len(blocks))
		for i, b := range  blocks {
			blockIDs[i] = b.Header.ID().String()
			backend.Mock.On("GetBlockByID", mocks.Anything, b.ID()).Return(b, nil)
			backend.Mock.On("GetExecutionResultForBlockID", mocks.Anything, b.ID()).Return(executionResults[i], nil)
 		}

		req, err := http.NewRequest("GET", blockURL(blockIDs, 0, 0, true, true), nil)
		require.NoError(t, err)

		rr := executeRequest(req, backend)

		expected := expectedBlockResponsesExpanded(blocks, executionResults)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.JSONEq(t, expected, rr.Body.String())
	})

}

func expectedBlockResponsesExpanded(blocks []*flow.Block, execResult []*flow.ExecutionResult) string {
	blockResponses := make([]string, len(blocks))
	for i, b := range blocks {
		blockResponses[i] = expectedBlockResponseExpanded(b, execResult[i])
	}
	return fmt.Sprintf("[%s]",strings.Join(blockResponses, ","))
}

func expectedBlockResponseExpanded(block *flow.Block, execResult *flow.ExecutionResult) string {
	id := block.ID().String()
	execResultID := execResult.ID().String()
	execLink := fmt.Sprintf("/v1/execution_results/%s", execResultID)
	blockLink := fmt.Sprintf("/v1/blocks/%s", id)

	timestamp := block.Header.Timestamp.Format(time.RFC3339Nano)

	return fmt.Sprintf(`
	{
		"header": {
			"id": "%s",
			"parent_id": "%s",
			"height": %d,
			"timestamp": "%s",
			"parent_voter_signature": "%s"
		},
		"payload": {
			"collection_guarantees": [],
			"block_seals": []
		},
		"execution_result": {
			"id": "%s",
			"block_id": "%s",
			"events": [],
			"_links": {
				"_self": "%s"
			}
		},
		"_expandable": {},
		"_links": {
			"_self": "%s"
		}
	}`, id, block.Header.ParentID.String(), int32(block.Header.Height), timestamp,
	toBase64(block.Header.ParentVoterSigData), execResultID, execResult.BlockID, execLink, blockLink)
}
