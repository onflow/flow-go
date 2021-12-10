package rest

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	mocks "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/rest/middleware"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type testVector struct {
	description      string
	request          *http.Request
	expectedStatus   int
	expectedResponse string
}

// TestGetBlocks tests the get blocks by ID and get blocks by heights API
func TestGetBlocks(t *testing.T) {
	backend := &mock.API{}

	blkCnt := 10
	blockIDs, heights, blocks, executionResults := generateMocks(backend, blkCnt)

	singleBlockExpandedResponse := expectedBlockResponsesExpanded(blocks[:2], executionResults[:2], true)
	multipleBlockExpandedResponse := expectedBlockResponsesExpanded(blocks, executionResults, true)

	singleBlockCondensedResponse := expectedBlockResponsesExpanded(blocks[:2], executionResults[:2], false)
	multipleBlockCondensedResponse := expectedBlockResponsesExpanded(blocks, executionResults, false)

	invalidID := unittest.IdentifierFixture().String()
	invalidHeight := fmt.Sprintf("%d", blkCnt+1)

	maxIDs := flow.IdentifierList(unittest.IdentifierListFixture(MaxAllowedBlockIDs + 1))

	testVectors := []testVector{
		{
			description:      "Get single expanded block by ID",
			request:          getByIDsExpandedURL(t, blockIDs[:2]),
			expectedStatus:   http.StatusOK,
			expectedResponse: singleBlockExpandedResponse,
		},
		{
			description:      "Get multiple expanded blocks by IDs",
			request:          getByIDsExpandedURL(t, blockIDs),
			expectedStatus:   http.StatusOK,
			expectedResponse: multipleBlockExpandedResponse,
		},
		{
			description:      "Get single condensed block by ID",
			request:          getByIDsCondensedURL(t, blockIDs[:2]),
			expectedStatus:   http.StatusOK,
			expectedResponse: singleBlockCondensedResponse,
		},
		{
			description:      "Get multiple condensed blocks by IDs",
			request:          getByIDsCondensedURL(t, blockIDs),
			expectedStatus:   http.StatusOK,
			expectedResponse: multipleBlockCondensedResponse,
		},
		{
			description:      "Get single expanded block by height",
			request:          getByHeightsExpandedURL(t, heights[:2]...),
			expectedStatus:   http.StatusOK,
			expectedResponse: singleBlockExpandedResponse,
		},
		{
			description:      "Get multiple expanded blocks by heights",
			request:          getByHeightsExpandedURL(t, heights...),
			expectedStatus:   http.StatusOK,
			expectedResponse: multipleBlockExpandedResponse,
		},
		{
			description:      "Get multiple expanded blocks by start and end height",
			request:          getByStartEndHeightExpandedURL(t, heights[0], heights[len(heights)-1]),
			expectedStatus:   http.StatusOK,
			expectedResponse: multipleBlockExpandedResponse,
		},
		{
			description:      "Get block by ID not found",
			request:          getByIDsExpandedURL(t, []string{invalidID}),
			expectedStatus:   http.StatusNotFound,
			expectedResponse: fmt.Sprintf(`{"code":404, "message":"error looking up block with ID %s"}`, invalidID),
		},
		{
			description:      "Get block by height not found",
			request:          getByHeightsExpandedURL(t, invalidHeight),
			expectedStatus:   http.StatusNotFound,
			expectedResponse: fmt.Sprintf(`{"code":404, "message":"error looking up block at height %s"}`, invalidHeight),
		},
		{
			description:      "Get block by end height less than start height",
			request:          getByStartEndHeightExpandedURL(t, heights[len(heights)-1], heights[0]),
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: `{"code":400, "message": "start height must be less than or equal to end height"}`,
		},
		{
			description:      "Get block by both heights and start and end height",
			request:          requestURL(t, nil, heights[len(heights)-1], heights[0], true, heights...),
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: `{"code":400, "message": "can only provide either heights or start and end height range"}`,
		},
		{
			description:      "Get block with missing height param",
			request:          getByHeightsExpandedURL(t), // no height query param specified
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: `{"code":400, "message": "must provide either heights or start and end height range"}`,
		},
		{
			description:      "Get block with missing height values",
			request:          getByHeightsExpandedURL(t, ""), // height query param specified with no value
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: `{"code":400, "message": "must provide either heights or start and end height range"}`,
		},
		{
			description:      "Get block by more than maximum permissible number of IDs",
			request:          getByHeightsExpandedURL(t, maxIDs.Strings()...), // height query param specified with no value
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: fmt.Sprintf(`{"code":400, "message": "invalid height specified: at most %d heights can be requested at a time"}`, MaxAllowedIDs),
		},
	}

	for _, tv := range testVectors {
		responseRec := executeRequest(tv.request, backend)
		require.Equal(t, tv.expectedStatus, responseRec.Code, "failed test %s: incorrect response code", tv.description)
		actualResp := responseRec.Body.String()
		require.JSONEq(t, tv.expectedResponse, actualResp, "Failed: %s: incorrect response body", tv.description)
	}

}

func requestURL(t *testing.T, ids []string, start string, end string, expandResponse bool, heights ...string) *http.Request {
	u, _ := url.Parse("/v1/blocks")
	q := u.Query()

	if len(ids) > 0 {
		u, _ = url.Parse(u.String() + "/" + strings.Join(ids, ","))
	}

	if start != "" {
		q.Add(startHeightQueryParam, start)
		q.Add(endHeightQueryParam, end)
	}

	if len(heights) > 0 {
		heightsStr := strings.Join(heights, ",")
		q.Add(heightQueryParam, heightsStr)
	}

	if expandResponse {
		var expands []string
		expands = append(expands, ExpandableFieldPayload)
		expands = append(expands, ExpandableExecutionResult)
		expandsStr := strings.Join(expands, ",")
		q.Add(middleware.ExpandQueryParam, expandsStr)
	}

	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	require.NoError(t, err)
	return req
}

func getByIDsExpandedURL(t *testing.T, ids []string) *http.Request {
	return requestURL(t, ids, "", "", true)
}

func getByHeightsExpandedURL(t *testing.T, heights ...string) *http.Request {
	return requestURL(t, nil, "", "", true, heights...)
}

func getByStartEndHeightExpandedURL(t *testing.T, start, end string) *http.Request {
	return requestURL(t, nil, start, end, true)
}

func getByIDsCondensedURL(t *testing.T, ids []string) *http.Request {
	return requestURL(t, ids, "", "", false)
}

func generateMocks(backend *mock.API, count int) ([]string, []string, []*flow.Block, []*flow.ExecutionResult) {
	blockIDs := make([]string, count)
	heights := make([]string, count)
	blocks := make([]*flow.Block, count)
	executionResults := make([]*flow.ExecutionResult, count)

	for i := 0; i < count; i++ {
		block := unittest.BlockFixture()
		block.Header.Height = uint64(i)
		blocks[i] = &block
		blockIDs[i] = block.Header.ID().String()
		heights[i] = fmt.Sprintf("%d", block.Header.Height)

		executionResult := unittest.ExecutionResultFixture()
		executionResult.BlockID = block.ID()
		executionResults[i] = executionResult

		backend.Mock.On("GetBlockByID", mocks.Anything, block.ID()).Return(&block, nil)
		backend.Mock.On("GetBlockByHeight", mocks.Anything, block.Header.Height).Return(&block, nil)
		backend.Mock.On("GetExecutionResultForBlockID", mocks.Anything, block.ID()).Return(executionResults[i], nil)
	}

	// any other call to the backend should return a not found error
	backend.Mock.On("GetBlockByID", mocks.Anything, mocks.Anything).Return(nil, status.Error(codes.NotFound, "not found"))
	backend.Mock.On("GetBlockByHeight", mocks.Anything, mocks.Anything).Return(nil, status.Error(codes.NotFound, "not found"))

	return blockIDs, heights, blocks, executionResults
}

func expectedBlockResponsesExpanded(blocks []*flow.Block, execResult []*flow.ExecutionResult, expanded bool) string {
	blockResponses := make([]string, len(blocks))
	for i, b := range blocks {
		blockResponses[i] = expectedBlockResponse(b, execResult[i], expanded)
	}
	return fmt.Sprintf("[%s]", strings.Join(blockResponses, ","))
}

func expectedBlockResponse(block *flow.Block, execResult *flow.ExecutionResult, expanded bool) string {
	id := block.ID().String()
	execResultID := execResult.ID().String()
	execLink := fmt.Sprintf("/v1/execution_results/%s", execResultID)
	blockLink := fmt.Sprintf("/v1/blocks/%s", id)
	payloadLink := fmt.Sprintf("/v1/blocks/%s/payload", id)

	timestamp := block.Header.Timestamp.Format(time.RFC3339Nano)

	if expanded {
		return fmt.Sprintf(`
	{
		"header": {
			"id": "%s",
			"parent_id": "%s",
			"height": "%d",
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
	}`, id, block.Header.ParentID.String(), block.Header.Height, timestamp,
			toBase64(block.Header.ParentVoterSigData), execResultID, execResult.BlockID, execLink, blockLink)
	}

	return fmt.Sprintf(`
	{
		"header": {
			"id": "%s",
			"parent_id": "%s",
			"height": "%d",
			"timestamp": "%s",
			"parent_voter_signature": "%s"
		},
		"_expandable": {
            "payload": "%s",
            "execution_result": "%s"
        },
		"_links": {
			"_self": "%s"
		}
	}`, id, block.Header.ParentID.String(), block.Header.Height, timestamp,
		toBase64(block.Header.ParentVoterSigData), payloadLink, execLink, blockLink)
}
