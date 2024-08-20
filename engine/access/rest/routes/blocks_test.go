package routes

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/engine/access/rest/util"

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

func prepareTestVectors(t *testing.T,
	blockIDs []string,
	heights []string,
	blocks []*flow.Block,
	executionResults []*flow.ExecutionResult,
	blkCnt int) []testVector {

	singleBlockExpandedResponse := expectedBlockResponsesExpanded(blocks[:1], executionResults[:1], true, flow.BlockStatusUnknown)
	singleSealedBlockExpandedResponse := expectedBlockResponsesExpanded(blocks[:1], executionResults[:1], true, flow.BlockStatusSealed)
	multipleBlockExpandedResponse := expectedBlockResponsesExpanded(blocks, executionResults, true, flow.BlockStatusUnknown)
	multipleSealedBlockExpandedResponse := expectedBlockResponsesExpanded(blocks, executionResults, true, flow.BlockStatusSealed)

	singleBlockCondensedResponse := expectedBlockResponsesExpanded(blocks[:1], executionResults[:1], false, flow.BlockStatusUnknown)
	multipleBlockCondensedResponse := expectedBlockResponsesExpanded(blocks, executionResults, false, flow.BlockStatusUnknown)

	multipleBlockHeaderWithHeaderSelectedResponse := expectedBlockResponsesSelected(blocks, executionResults, flow.BlockStatusUnknown, []string{"header"})
	multipleBlockHeaderWithHeaderAndStatusSelectedResponse := expectedBlockResponsesSelected(blocks, executionResults, flow.BlockStatusUnknown, []string{"header", "block_status"})
	multipleBlockHeaderWithUnknownSelectedResponse := expectedBlockResponsesSelected(blocks, executionResults, flow.BlockStatusUnknown, []string{"unknown"})

	invalidID := unittest.IdentifierFixture().String()
	invalidHeight := fmt.Sprintf("%d", blkCnt+1)

	maxIDs := flow.IdentifierList(unittest.IdentifierListFixture(request.MaxBlockRequestHeightRange + 1))

	testVectors := []testVector{
		{
			description:      "Get single expanded block by ID",
			request:          getByIDsExpandedURL(t, blockIDs[:1]),
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
			request:          getByIDsCondensedURL(t, blockIDs[:1]),
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
			request:          getByHeightsExpandedURL(t, heights[:1]...),
			expectedStatus:   http.StatusOK,
			expectedResponse: singleSealedBlockExpandedResponse,
		},
		{
			description:      "Get multiple expanded blocks by heights",
			request:          getByHeightsExpandedURL(t, heights...),
			expectedStatus:   http.StatusOK,
			expectedResponse: multipleSealedBlockExpandedResponse,
		},
		{
			description:      "Get multiple expanded blocks by start and end height",
			request:          getByStartEndHeightExpandedURL(t, heights[0], heights[len(heights)-1]),
			expectedStatus:   http.StatusOK,
			expectedResponse: multipleSealedBlockExpandedResponse,
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
			request:          requestURL(t, nil, heights[len(heights)-1], heights[0], true, []string{}, heights...),
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
			request:          getByIDsCondensedURL(t, maxIDs.Strings()), // height query param specified with no value
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: fmt.Sprintf(`{"code":400, "message": "at most %d IDs can be requested at a time"}`, request.MaxBlockRequestHeightRange),
		},
		{
			description:      "Get multiple blocks by IDs with header selected",
			request:          getByIDsCondensedWithSelectURL(t, blockIDs, []string{"header"}),
			expectedStatus:   http.StatusOK,
			expectedResponse: multipleBlockHeaderWithHeaderSelectedResponse,
		},
		{
			description:      "Get multiple blocks by IDs with header and block_status selected",
			request:          getByIDsCondensedWithSelectURL(t, blockIDs, []string{"header", "block_status"}),
			expectedStatus:   http.StatusOK,
			expectedResponse: multipleBlockHeaderWithHeaderAndStatusSelectedResponse,
		},
		{
			description:      "Get multiple blocks by IDs with unknown select object",
			request:          getByIDsCondensedWithSelectURL(t, blockIDs, []string{"unknown"}),
			expectedStatus:   http.StatusOK,
			expectedResponse: multipleBlockHeaderWithUnknownSelectedResponse,
		},
	}
	return testVectors
}

// TestGetBlocks tests local get blocks by ID and get blocks by heights API
func TestAccessGetBlocks(t *testing.T) {
	backend := &mock.API{}

	blkCnt := 10
	blockIDs, heights, blocks, executionResults := generateMocks(backend, blkCnt)
	testVectors := prepareTestVectors(t, blockIDs, heights, blocks, executionResults, blkCnt)

	for _, tv := range testVectors {
		rr := executeRequest(tv.request, backend)
		require.Equal(t, tv.expectedStatus, rr.Code, "failed test %s: incorrect response code", tv.description)
		actualResp := rr.Body.String()
		require.JSONEq(t, tv.expectedResponse, actualResp, "Failed: %s: incorrect response body", tv.description)
	}
}

func requestURL(t *testing.T, ids []string, start string, end string, expandResponse bool, selectedFields []string, heights ...string) *http.Request {
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

	if len(selectedFields) > 0 {
		selectedStr := strings.Join(selectedFields, ",")
		q.Add(middleware.SelectQueryParam, selectedStr)
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
	return requestURL(t, ids, "", "", true, []string{})
}

func getByHeightsExpandedURL(t *testing.T, heights ...string) *http.Request {
	return requestURL(t, nil, "", "", true, []string{}, heights...)
}

func getByStartEndHeightExpandedURL(t *testing.T, start, end string) *http.Request {
	return requestURL(t, nil, start, end, true, []string{})
}

func getByIDsCondensedURL(t *testing.T, ids []string) *http.Request {
	return requestURL(t, ids, "", "", false, []string{})
}

func getByIDsCondensedWithSelectURL(t *testing.T, ids []string, selectedFields []string) *http.Request {
	return requestURL(t, ids, "", "", false, selectedFields)
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

		backend.Mock.On("GetBlockByID", mocks.Anything, block.ID()).Return(&block, flow.BlockStatusSealed, nil)
		backend.Mock.On("GetBlockByHeight", mocks.Anything, block.Header.Height).Return(&block, flow.BlockStatusSealed, nil)
		backend.Mock.On("GetExecutionResultForBlockID", mocks.Anything, block.ID()).Return(executionResults[i], nil)
	}

	// any other call to the backend should return a not found error
	backend.Mock.On("GetBlockByID", mocks.Anything, mocks.Anything).Return(nil, flow.BlockStatusUnknown, status.Error(codes.NotFound, "not found"))
	backend.Mock.On("GetBlockByHeight", mocks.Anything, mocks.Anything).Return(nil, flow.BlockStatusUnknown, status.Error(codes.NotFound, "not found"))

	return blockIDs, heights, blocks, executionResults
}

func expectedBlockResponsesExpanded(
	blocks []*flow.Block,
	execResult []*flow.ExecutionResult,
	expanded bool,
	status flow.BlockStatus,
	selectedFields ...string,
) string {
	blockResponses := make([]string, 0)
	for i, b := range blocks {
		response := expectedBlockResponse(b, execResult[i], expanded, status, selectedFields...)
		if response != "" {
			blockResponses = append(blockResponses, response)
		}
	}
	return fmt.Sprintf("[%s]", strings.Join(blockResponses, ","))
}

func expectedBlockResponsesSelected(
	blocks []*flow.Block,
	execResult []*flow.ExecutionResult,
	status flow.BlockStatus,
	selectedFields []string,
) string {
	return expectedBlockResponsesExpanded(blocks, execResult, false, status, selectedFields...)
}

func expectedBlockResponse(
	block *flow.Block,
	execResult *flow.ExecutionResult,
	expanded bool,
	status flow.BlockStatus,
	selectedFields ...string,
) string {
	id := block.ID().String()
	execResultID := execResult.ID().String()
	blockLink := fmt.Sprintf("/v1/blocks/%s", id)
	payloadLink := fmt.Sprintf("/v1/blocks/%s/payload", id)
	execLink := fmt.Sprintf("/v1/execution_results/%s", execResultID)
	timestamp := block.Header.Timestamp.Format(time.RFC3339Nano)

	header := fmt.Sprintf(`"header": {
		"id": "%s",
		"parent_id": "%s",
		"height": "%d",
		"timestamp": "%s",
		"parent_voter_signature": "%s"
	}`, id, block.Header.ParentID.String(), block.Header.Height, timestamp, util.ToBase64(block.Header.ParentVoterSigData))

	links := fmt.Sprintf(`"_links": {
		"_self": "%s"
	}`, blockLink)

	expandable := fmt.Sprintf(`"_expandable": {
		"payload": "%s",
		"execution_result": "%s"
	}`, payloadLink, execLink)

	blockStatus := fmt.Sprintf(`"block_status": "%s"`, status.String())
	payload := `"payload": {"collection_guarantees": [],"block_seals": []}`
	executionResult := fmt.Sprintf(`"execution_result": %s`, executionResultExpectedStr(execResult))

	partsSet := make(map[string]string)

	if expanded {
		partsSet["header"] = header
		partsSet["payload"] = payload
		partsSet["executionResult"] = executionResult
		partsSet["_expandable"] = `"_expandable": {}`
		partsSet["_links"] = links
		partsSet["block_status"] = blockStatus
	} else {
		partsSet["header"] = header
		partsSet["_expandable"] = expandable
		partsSet["_links"] = links
		partsSet["block_status"] = blockStatus
	}

	if len(selectedFields) > 0 {
		// filter a json struct
		// elements whose keys are not found in the filter map will be removed
		selectedFieldSet := make(map[string]struct{}, len(selectedFields))
		for _, field := range selectedFields {
			selectedFieldSet[field] = struct{}{}
		}

		for key := range partsSet {
			if _, found := selectedFieldSet[key]; !found {
				delete(partsSet, key)
			}
		}
	}

	// Iterate over the map and append the values to the slice
	var values []string
	for _, value := range partsSet {
		values = append(values, value)
	}
	if len(values) == 0 {
		return ""
	}

	return fmt.Sprintf("{%s}", strings.Join(values, ","))
}
