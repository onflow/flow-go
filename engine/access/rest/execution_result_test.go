package rest

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	mocks "github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access/mock"
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
		id := unittest.IdentifierFixture()
		result := unittest.ExecutionResultFixture()

		backend.Mock.
			On("GetExecutionResultByID", mocks.Anything, id).
			Return(result, nil).
			Once()

		req := getResultByIDReq(id.String(), nil)
		expected := fmt.Sprintf(`{
			"id": "%s",
			"block_id": "%s",
			"events": [],
			"_links": {
				"_self": "/v1/execution_results/%s"
			}
		}`, result.ID(), result.BlockID, result.ID())
		assertOKResponse(t, req, expected, backend)
		mocks.AssertExpectationsForObjects(t, backend)
	})

	t.Run("get by ID with events", func(t *testing.T) {
		backend := &mock.API{}
		id := unittest.IdentifierFixture()
		result := unittest.ExecutionResultFixture(unittest.WithServiceEvents(1))

		backend.Mock.
			On("GetExecutionResultByID", mocks.Anything, id).
			Return(result, nil).
			Once()

		req := getResultByIDReq(id.String(), nil)
		expected := fmt.Sprintf(`{
			"id": "%s",
			"block_id": "%s",
			"events": [{
				"type": "%s"
			}],
			"_links": {
				"_self": "/v1/execution_results/%s"
			}
		}`, result.ID(), result.BlockID, result.ServiceEvents[0].Type, result.ID())
		assertOKResponse(t, req, expected, backend)
		mocks.AssertExpectationsForObjects(t, backend)
	})

	t.Run("get by ID not found", func(t *testing.T) {
		backend := &mock.API{}
		id := unittest.IdentifierFixture()
		backend.Mock.
			On("GetExecutionResultByID", mocks.Anything, id).
			Return(nil, status.Error(codes.NotFound, "not found")).
			Once()

		req := getResultByIDReq(id.String(), nil)
		assertResponse(t, req, http.StatusNotFound, `{"code":404,"message":"not found"}`, backend)
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
		expected := fmt.Sprintf(`[{
			"id": "%s",
			"block_id": "%s",
			"events": [],
			"_links": {
				"_self": "/v1/execution_results/%s"
			}
		}]`, result.ID(), result.BlockID, result.ID())
		assertOKResponse(t, req, expected, backend)
		mocks.AssertExpectationsForObjects(t, backend)
	})

	t.Run("get by block ID not found", func(t *testing.T) {
		backend := &mock.API{}
		blockID := unittest.IdentifierFixture()
		backend.Mock.
			On("GetExecutionResultForBlockID", mocks.Anything, blockID).
			Return(nil, status.Error(codes.NotFound, "not found")).
			Once()

		req := getResultByIDReq("", []string{blockID.String()})
		assertResponse(t, req, http.StatusNotFound, `{"code":404,"message":"not found"}`, backend)
		mocks.AssertExpectationsForObjects(t, backend)
	})
}
