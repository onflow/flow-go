package rest

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/model/flow"
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

func executeRequest(req *http.Request, backend *mock.API) (*httptest.ResponseRecorder, error) {
	var b bytes.Buffer
	logger := zerolog.New(&b)
	router, err := newRouter(backend, logger, flow.Canary.Chain())
	if err != nil {
		return nil, err
	}

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr, nil
}

func assertOKResponse(t *testing.T, req *http.Request, expectedRespBody string, backend *mock.API) {
	assertResponse(t, req, http.StatusOK, expectedRespBody, backend)
}

func assertResponse(t *testing.T, req *http.Request, status int, expectedRespBody string, backend *mock.API) {
	rr, err := executeRequest(req, backend)
	assert.NoError(t, err)

	actualResponseBody := rr.Body.String()
	require.JSONEq(t,
		expectedRespBody,
		actualResponseBody,
		fmt.Sprintf("Failed Request: %s\nExpected JSON:\n %s \nActual JSON:\n %s\n", req.URL, expectedRespBody, actualResponseBody),
	)
	require.Equal(t, status, rr.Code)
}
