package rest

import (
	"bytes"
	"fmt"
	"github.com/onflow/flow-go/access/mock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	ExpandableFieldPayload    = "payload"
	ExpandableExecutionResult = "execution_result"
	sealedHeightQueryParam    = "sealed"
	finalHeightQueryParam     = "final"
	startHeightQueryParam     = "start_height"
	endHeightQueryParam       = "end_height"
	heightQueryParam          = "height"
)

func executeRequest(req *http.Request, backend *mock.API) *httptest.ResponseRecorder {
	var b bytes.Buffer
	logger := zerolog.New(&b)
	router := initRouter(backend, logger)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	return rr
}

func assertOKResponse(t *testing.T, req *http.Request, expectedRespBody string, backend *mock.API) {
	assertResponse(t, req, http.StatusOK, expectedRespBody, backend)
}

func assertResponse(t *testing.T, req *http.Request, status int, expectedRespBody string, backend *mock.API) {
	rr := executeRequest(req, backend)
	require.Equal(t, status, rr.Code)
	fmt.Println(rr.Body.String())
	actualResponseBody := rr.Body.String()
	require.JSONEq(t, expectedRespBody, actualResponseBody, fmt.Sprintf("failed for req: %s", req.URL))
}
