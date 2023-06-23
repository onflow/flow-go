package tests

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
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/onflow/flow-go/engine/access/rest/api"
	restproxy "github.com/onflow/flow-go/engine/access/rest/apiproxy"
	restmock "github.com/onflow/flow-go/engine/access/rest/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
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

func executeRequest(req *http.Request, restHandler api.RestServerApi) (*httptest.ResponseRecorder, error) {
	var b bytes.Buffer
	logger := zerolog.New(&b)

	router, err := rest.NewRouter(restHandler, logger, flow.Testnet.Chain(), metrics.NewNoopCollector())
	if err != nil {
		return nil, err
	}

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr, nil
}

func newAccessRestHandler(backend *mock.API) api.RestServerApi {
	var b bytes.Buffer
	logger := zerolog.New(&b)

	return rest.NewServerRequestHandler(logger, backend)
}

func newObserverRestHandler(backend *mock.API, restForwarder *restmock.RestServerApi) (api.RestServerApi, error) {
	var b bytes.Buffer
	logger := zerolog.New(&b)
	observerCollector := metrics.NewNoopCollector()

	return &restproxy.RestRouter{
		Logger:   logger,
		Metrics:  observerCollector,
		Upstream: restForwarder,
		Observer: rest.NewServerRequestHandler(logger, backend),
	}, nil
}

func assertOKResponse(t *testing.T, req *http.Request, expectedRespBody string, restHandler api.RestServerApi) {
	assertResponse(t, req, http.StatusOK, expectedRespBody, restHandler)
}

func assertResponse(t *testing.T, req *http.Request, status int, expectedRespBody string, restHandler api.RestServerApi) {
	rr, err := executeRequest(req, restHandler)
	assert.NoError(t, err)
	actualResponseBody := rr.Body.String()
	require.JSONEq(t,
		expectedRespBody,
		actualResponseBody,
		fmt.Sprintf("Failed Request: %s\nExpected JSON:\n %s \nActual JSON:\n %s\n", req.URL, expectedRespBody, actualResponseBody),
	)
	require.Equal(t, status, rr.Code)
}
