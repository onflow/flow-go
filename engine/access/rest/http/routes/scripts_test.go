package routes_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	mocks "github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/rest/router"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

var validCode = []byte(`access(all) fun main(foo: String): String { return foo }`)
var validArgs = []byte(`{ "type": "String", "value": "hello world" }`)
var validBody = map[string]interface{}{
	"script":    util.ToBase64(validCode),
	"arguments": []string{util.ToBase64(validArgs)},
}

// requestScriptParams represents the set of query parameters and body values
// used to construct a script execution request against the /v1/scripts endpoint.
// Each field corresponds to a query parameter or request body field that the
// ExecuteScript handler supports
type requestScriptParams struct {
	id                      string
	height                  string
	agreeingExecutorsCount  string
	requiredExecutors       []string
	includeExecutorMetadata string
	body                    interface{}
}

// TestScripts_HappyPath verifies that the ExecuteScript handler returns correct
// responses when scripts are successfully executed against different block targets.
// It covers both "legacyParams" behavior (returning only the value) and the
// new path (returning an ExecuteScriptResponse when execution parameters are set).
//
// Test cases:
//  1. Execute at specific block ID (legacyParams, expect only value).
//  2. Execute at latest block height (legacyParams, expect only value).
//  3. Execute at specific block height (legacyParams, expect only value).
//  4. Execute at final block height (legacyParams, expect only value).
//  5. Execute at specific block ID (with metadata, expect ExecuteScriptResponse).
//  6. Execute at specific block height (with metadata, expect ExecuteScriptResponse).
//  7. Execute at latest block height (with metadata, expect ExecuteScriptResponse).
//
// TODO: legacyParams is only to temporarily support current behaviour.
// In the new API version (e.g. /v2), we should delete legacy cases, update buildScriptResponse helper func and
// update godoc once the new unified ExecuteScriptResponse behavior is enforced.
func TestScripts_HappyPath(t *testing.T) {
	t.Run("block ID (legacyParams)", func(t *testing.T) {
		backend := mock.NewAPI(t)
		blockID := unittest.IdentifierFixture()
		backend.On("ExecuteScriptAtBlockID", mocks.Anything, blockID, validCode, [][]byte{validArgs}, mocks.Anything).
			Return([]byte("hello world"), &access.ExecutorMetadata{}, nil).
			Once()

		req := buildScriptRequest(
			requestScriptParams{
				id:   blockID.String(),
				body: validBody,
			},
		)
		expectedResp := buildScriptResponse(
			[]byte(`hello world`),
			nil,
		)
		router.AssertOKResponse(t, req, expectedResp, backend)
	})

	t.Run("block height (legacyParams)", func(t *testing.T) {
		backend := mock.NewAPI(t)
		height := uint64(1337)
		backend.On("ExecuteScriptAtBlockHeight", mocks.Anything, height, validCode, [][]byte{validArgs}, mocks.Anything).
			Return([]byte("hello world"), &access.ExecutorMetadata{}, nil).
			Once()

		req := buildScriptRequest(
			requestScriptParams{
				height: fmt.Sprintf("%d", height),
				body:   validBody,
			},
		)
		expectedResp := buildScriptResponse(
			[]byte(`hello world`),
			nil,
		)
		router.AssertOKResponse(t, req, expectedResp, backend)
	})

	t.Run("latest height (legacyParams)", func(t *testing.T) {
		backend := mock.NewAPI(t)
		backend.On("ExecuteScriptAtLatestBlock", mocks.Anything, validCode, [][]byte{validArgs}, mocks.Anything).
			Return([]byte("hello world"), &access.ExecutorMetadata{}, nil).
			Once()

		req := buildScriptRequest(
			requestScriptParams{
				height: router.SealedHeightQueryParam,
				body:   validBody,
			},
		)
		expectedResp := buildScriptResponse(
			[]byte(`hello world`),
			nil,
		)
		router.AssertOKResponse(t, req, expectedResp, backend)
	})

	t.Run("final height (legacyParams)", func(t *testing.T) {
		backend := mock.NewAPI(t)
		finalBlock := unittest.BlockHeaderFixture()
		backend.On("GetLatestBlockHeader", mocks.Anything, false).
			Return(finalBlock, flow.BlockStatusFinalized, nil).
			Once()
		backend.On("ExecuteScriptAtBlockHeight", mocks.Anything, finalBlock.Height, validCode, [][]byte{validArgs}, mocks.Anything).
			Return([]byte("hello world"), &access.ExecutorMetadata{}, nil).
			Once()

		req := buildScriptRequest(
			requestScriptParams{
				height: router.FinalHeightQueryParam,
				body:   validBody,
			},
		)
		expectedResp := buildScriptResponse(
			[]byte(`hello world`),
			nil,
		)
		router.AssertOKResponse(t, req, expectedResp, backend)
	})

	metadata := &access.ExecutorMetadata{
		ExecutionResultID: unittest.IdentifierFixture(),
		ExecutorIDs:       unittest.IdentifierListFixture(2),
	}

	t.Run("latest height (with metadata)", func(t *testing.T) {
		backend := mock.NewAPI(t)

		backend.On("ExecuteScriptAtLatestBlock", mocks.Anything, validCode, [][]byte{validArgs}, mocks.Anything).
			Return([]byte("hello world"), metadata, nil).
			Once()

		req := buildScriptRequest(
			requestScriptParams{
				height:                  router.SealedHeightQueryParam,
				includeExecutorMetadata: "true",
				body:                    validBody,
			},
		)

		expectedResp := buildScriptResponse(
			[]byte(`hello world`),
			metadata,
		)
		router.AssertOKResponse(t, req, expectedResp, backend)
	})

	t.Run("block height (with metadata)", func(t *testing.T) {
		backend := mock.NewAPI(t)
		height := uint64(1337)

		backend.On("ExecuteScriptAtBlockHeight", mocks.Anything, height, validCode, [][]byte{validArgs}, mocks.Anything).
			Return([]byte("hello world"), metadata, nil).
			Once()

		req := buildScriptRequest(
			requestScriptParams{
				height:                  fmt.Sprintf("%d", height),
				includeExecutorMetadata: "true",
				body:                    validBody,
			},
		)

		expectedResp := buildScriptResponse(
			[]byte(`hello world`),
			metadata,
		)
		router.AssertOKResponse(t, req, expectedResp, backend)
	})

	t.Run("block ID (with metadata)", func(t *testing.T) {
		backend := mock.NewAPI(t)

		blockID := unittest.IdentifierFixture()
		backend.On("ExecuteScriptAtBlockID", mocks.Anything, blockID, validCode, [][]byte{validArgs}, mocks.Anything).
			Return([]byte("hello world"), metadata, nil).
			Once()

		req := buildScriptRequest(
			requestScriptParams{
				id:                      blockID.String(),
				includeExecutorMetadata: "true",
				body:                    validBody,
			},
		)

		expectedResp := buildScriptResponse(
			[]byte(`hello world`),
			metadata,
		)
		router.AssertOKResponse(t, req, expectedResp, backend)
	})
}

// TestScripts_Errors verifies that the script execution endpoints correctly handle
// invalid inputs and backend errors by returning the expected HTTP error responses.
//
// Test cases:
//  1. Invalid script arguments (bad block ID format), expect 400 Bad Request.
//  2. Backend error when executing at a specific block ID, expect 400 Bad Request.
//  3. Backend error when executing at a specific block height, expect 400 Bad Request.
//  4. Backend error when executing at the latest block, expect 400 Bad Request.
func TestScripts_Errors(t *testing.T) {
	t.Run("invalid arguments", func(t *testing.T) {
		backend := mock.NewAPI(t)

		req := buildScriptRequest(
			requestScriptParams{
				id:   "invalidID",
				body: validBody,
			},
		)

		expectedResp := `{"code":400, "message":"invalid ID format"}`
		router.AssertResponse(t, req, http.StatusBadRequest, expectedResp, backend)
	})

	t.Run("backend error at block ID", func(t *testing.T) {
		backend := mock.NewAPI(t)
		blockID := unittest.IdentifierFixture()

		req := buildScriptRequest(
			requestScriptParams{
				id:   blockID.String(),
				body: validBody,
			},
		)

		backend.On("ExecuteScriptAtBlockID", mocks.Anything, blockID, validCode, [][]byte{validArgs}, mocks.Anything).
			Return(nil, &access.ExecutorMetadata{}, status.Error(codes.Internal, "internal server error")).
			Once()

		expectedResp := `{"code":500, "message":"rpc error: code = Internal desc = internal server error"}`
		router.AssertResponse(t, req, http.StatusInternalServerError, expectedResp, backend)
	})

	t.Run("backend error at block height", func(t *testing.T) {
		backend := mock.NewAPI(t)

		req := buildScriptRequest(
			requestScriptParams{
				height: "1337",
				body:   validBody,
			},
		)

		backend.On("ExecuteScriptAtBlockHeight", mocks.Anything, uint64(1337), validCode, [][]byte{validArgs}, mocks.Anything).
			Return(nil, &access.ExecutorMetadata{}, status.Error(codes.Internal, "internal server error")).
			Once()

		expectedResp := `{"code":500, "message":"rpc error: code = Internal desc = internal server error"}`
		router.AssertResponse(t, req, http.StatusInternalServerError, expectedResp, backend)
	})

	t.Run("backend error at latest block", func(t *testing.T) {
		backend := mock.NewAPI(t)

		req := buildScriptRequest(
			requestScriptParams{
				body: validBody,
			},
		)

		backend.On("ExecuteScriptAtLatestBlock", mocks.Anything, validCode, [][]byte{validArgs}, mocks.Anything).
			Return(nil, &access.ExecutorMetadata{}, status.Error(codes.Internal, "internal server error")).
			Once()

		expectedResp := `{"code":500, "message":"rpc error: code = Internal desc = internal server error"}`
		router.AssertResponse(t, req, http.StatusInternalServerError, expectedResp, backend)
	})
}

// buildScriptRequest builds an *http.Request for the /v1/scripts endpoint using
// the provided requestScriptParams. Query parameters are set based on the
// struct fields, and the request body is encoded as JSON. This helper is used
// in tests to simulate client requests to the ExecuteScript handler.
func buildScriptRequest(
	params requestScriptParams,
) *http.Request {
	u, _ := url.ParseRequestURI("/v1/scripts")
	q := u.Query()

	if params.id != "" {
		q.Add("block_id", params.id)
	}
	if params.height != "" {
		q.Add("block_height", params.height)
	}
	q.Add(router.AgreeingExecutorsCountQueryParam, params.agreeingExecutorsCount)
	q.Add(router.RequiredExecutorIdsQueryParam, strings.Join(params.requiredExecutors, ","))
	if len(params.includeExecutorMetadata) > 0 {
		q.Add(router.IncludeExecutorMetadataQueryParam, fmt.Sprint(params.includeExecutorMetadata))
	}

	u.RawQuery = q.Encode()

	jsonBody, _ := json.Marshal(params.body)
	req, _ := http.NewRequest("POST", u.String(), bytes.NewBuffer(jsonBody))

	return req
}

// buildScriptResponse builds the expected JSON response for ExecuteScript tests.
// If metadata is empty, only the value is returned (legacy behavior).
// Otherwise, the full ExecuteScriptResponse with executor metadata is returned.
func buildScriptResponse(value []byte, metadata *access.ExecutorMetadata) string {
	if metadata == nil {
		// legacyParams: only value is returned
		return fmt.Sprintf(`"%s"`, base64.StdEncoding.EncodeToString(value))
	}

	// new unified ExecuteScriptResponse
	expectedMetadata := map[string]interface{}{
		"execution_result_id": metadata.ExecutionResultID,
		"executor_ids":        metadata.ExecutorIDs,
	}
	metadataJSON, _ := json.Marshal(expectedMetadata)
	return fmt.Sprintf(`{"value":"%s","metadata":{"executor_metadata":%s}}`, value, string(metadataJSON))
}
