package routes

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	mocks "github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func getResultByIDReq(id string, blockIDs []string) *http.Request {
	endpoint := "/v1/execution_results"

	var u string
	if id != "" {
		u = fmt.Sprintf("%s/%s", endpoint, id)
	} else if len(blockIDs) > 0 {
		p, _ := url.Parse(endpoint)
		q := p.Query()
		q.Add("block_id", strings.Join(blockIDs, ","))
		p.RawQuery = q.Encode()
		u = p.String()
	}

	req, _ := http.NewRequest("GET", u, nil)
	return req
}

func TestGetResultByID(t *testing.T) {
	t.Run("get by ID", func(t *testing.T) {
		backend := &mock.API{}
		result := unittest.ExecutionResultFixture()
		id := unittest.IdentifierFixture()
		backend.Mock.
			On("GetExecutionResultByID", mocks.Anything, id).
			Return(result, nil).
			Once()
		req := getResultByIDReq(id.String(), nil)

		expected := executionResultExpectedStr(result)
		assertOKResponse(t, req, expected, backend)
		mocks.AssertExpectationsForObjects(t, backend)
	})

	t.Run("get by ID not found", func(t *testing.T) {
		backend := &mock.API{}
		id := unittest.IdentifierFixture()
		backend.Mock.
			On("GetExecutionResultByID", mocks.Anything, id).
			Return(nil, status.Error(codes.NotFound, "block not found")).
			Once()

		req := getResultByIDReq(id.String(), nil)
		assertResponse(t, req, http.StatusNotFound, `{"code":404,"message":"Flow resource not found: block not found"}`, backend)
		mocks.AssertExpectationsForObjects(t, backend)
	})
}

func TestGetResultBlockID(t *testing.T) {

	t.Run("get by block ID", func(t *testing.T) {
		backend := &mock.API{}
		blockID := unittest.IdentifierFixture()
		result := unittest.ExecutionResultFixture(unittest.WithExecutionResultBlockID(blockID))

		backend.Mock.
			On("GetExecutionResultForBlockID", mocks.Anything, blockID).
			Return(result, nil).
			Once()

		req := getResultByIDReq("", []string{blockID.String()})

		expected := fmt.Sprintf(`[%s]`, executionResultExpectedStr(result))
		assertOKResponse(t, req, expected, backend)
		mocks.AssertExpectationsForObjects(t, backend)
	})

	t.Run("get by block ID not found", func(t *testing.T) {
		backend := &mock.API{}
		blockID := unittest.IdentifierFixture()
		backend.Mock.
			On("GetExecutionResultForBlockID", mocks.Anything, blockID).
			Return(nil, status.Error(codes.NotFound, "block not found")).
			Once()

		req := getResultByIDReq("", []string{blockID.String()})
		assertResponse(t, req, http.StatusNotFound, `{"code":404,"message":"Flow resource not found: block not found"}`, backend)
		mocks.AssertExpectationsForObjects(t, backend)
	})
}

func executionResultExpectedStr(result *flow.ExecutionResult) string {
	chunks := make([]string, len(result.Chunks))
	for i, chunk := range result.Chunks {
		chunks[i] = fmt.Sprintf(`{
				"block_id": "%s",
				"collection_index": "%d",
				"start_state": "%s",
				"end_state": "%s",
				"number_of_transactions": "%d",
				"event_collection": "%s",
				"index": "%d",
				"total_computation_used": "%d"
			}`, chunk.BlockID, chunk.CollectionIndex, util.ToBase64(chunk.StartState[:]), util.ToBase64(chunk.EndState[:]),
			chunk.NumberOfTransactions, chunk.EventCollection.String(), chunk.Index, chunk.TotalComputationUsed)
	}
	chunksStr := fmt.Sprintf("[%s]", strings.Join(chunks, ","))
	expected := fmt.Sprintf(`{
			"id": "%s",
			"block_id": "%s",
			"events": [],
            "chunks": %s,
            "previous_result_id": "%s",
			"_links": {
				"_self": "/v1/execution_results/%s"
			}
		}`, result.ID(), result.BlockID, chunksStr, result.PreviousResultID.String(), result.ID())
	return expected
}
