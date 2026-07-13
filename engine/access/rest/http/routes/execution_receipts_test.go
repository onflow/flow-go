package routes_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	mocks "github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/rest/router"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func getReceiptsByBlockIDReq(blockID string) *http.Request {
	u := fmt.Sprintf("/v1/execution_receipts?block_id=%s", blockID)
	req, _ := http.NewRequest("GET", u, nil)
	return req
}

func getReceiptsByResultIDReq(resultID string) *http.Request {
	u := fmt.Sprintf("/v1/execution_receipts/results/%s", resultID)
	req, _ := http.NewRequest("GET", u, nil)
	return req
}

func executionReceiptExpectedStr(receipt *flow.ExecutionReceipt) string {
	spocks := make([]string, len(receipt.Spocks))
	for i, spock := range receipt.Spocks {
		spocks[i] = fmt.Sprintf(`"%s"`, util.ToBase64(spock))
	}
	spocksStr := fmt.Sprintf("[%s]", strings.Join(spocks, ","))

	resultID := receipt.ExecutionResult.ID()
	return fmt.Sprintf(`{
		"executor_id": "%s",
		"result_id": "%s",
		"spocks": %s,
		"executor_signature": "%s",
		"_expandable": {
			"execution_result": "/v1/execution_results/%s"
		}
	}`,
		receipt.ExecutorID,
		resultID,
		spocksStr,
		util.ToBase64(receipt.ExecutorSignature),
		resultID,
	)
}

func TestGetExecutionReceiptsByBlockID(t *testing.T) {
	t.Run("get by block_id", func(t *testing.T) {
		backend := &mock.API{}
		blockID := unittest.IdentifierFixture()
		receipt := unittest.ExecutionReceiptFixture()

		backend.Mock.
			On("GetExecutionReceiptsByBlockID", mocks.Anything, blockID).
			Return([]*flow.ExecutionReceipt{receipt}, nil).
			Once()

		req := getReceiptsByBlockIDReq(blockID.String())
		expected := fmt.Sprintf(`[%s]`, executionReceiptExpectedStr(receipt))
		router.AssertOKResponse(t, req, expected, backend)
		mocks.AssertExpectationsForObjects(t, backend)
	})

	t.Run("missing block_id parameter", func(t *testing.T) {
		backend := &mock.API{}
		req, _ := http.NewRequest("GET", "/v1/execution_receipts", nil)
		router.AssertResponse(t, req, http.StatusBadRequest, `{"code":400,"message":"block_id query parameter is required"}`, backend)
	})

	t.Run("block not found", func(t *testing.T) {
		backend := &mock.API{}
		blockID := unittest.IdentifierFixture()

		backend.Mock.
			On("GetExecutionReceiptsByBlockID", mocks.Anything, blockID).
			Return(nil, status.Error(codes.NotFound, "block not found")).
			Once()

		req := getReceiptsByBlockIDReq(blockID.String())
		router.AssertResponse(t, req, http.StatusNotFound, `{"code":404,"message":"Flow resource not found: block not found"}`, backend)
		mocks.AssertExpectationsForObjects(t, backend)
	})
}

func TestGetExecutionReceiptsByResultID(t *testing.T) {
	t.Run("get by result_id", func(t *testing.T) {
		backend := &mock.API{}
		resultID := unittest.IdentifierFixture()
		receipt := unittest.ExecutionReceiptFixture()

		backend.Mock.
			On("GetExecutionReceiptsByResultID", mocks.Anything, resultID).
			Return([]*flow.ExecutionReceipt{receipt}, nil).
			Once()

		req := getReceiptsByResultIDReq(resultID.String())
		expected := fmt.Sprintf(`[%s]`, executionReceiptExpectedStr(receipt))
		router.AssertOKResponse(t, req, expected, backend)
		mocks.AssertExpectationsForObjects(t, backend)
	})

	t.Run("result not found", func(t *testing.T) {
		backend := &mock.API{}
		resultID := unittest.IdentifierFixture()

		backend.Mock.
			On("GetExecutionReceiptsByResultID", mocks.Anything, resultID).
			Return(nil, status.Error(codes.NotFound, "result not found")).
			Once()

		req := getReceiptsByResultIDReq(resultID.String())
		router.AssertResponse(t, req, http.StatusNotFound, `{"code":404,"message":"Flow resource not found: result not found"}`, backend)
		mocks.AssertExpectationsForObjects(t, backend)
	})
}
