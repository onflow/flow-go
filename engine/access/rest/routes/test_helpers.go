package routes

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
	mock_state_stream "github.com/onflow/flow-go/engine/access/state_stream/mock"
	common_state_stream "github.com/onflow/flow-go/engine/common/state_stream"

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

func executeRequest(req *http.Request, backend *mock.API, stateStreamApi *mock_state_stream.API) (*httptest.ResponseRecorder, error) {
	var b bytes.Buffer
	logger := zerolog.New(&b)
	restCollector := metrics.NewNoopCollector()

	stateStreamConfig := common_state_stream.Config{
		EventFilterConfig: common_state_stream.DefaultEventFilterConfig,
		MaxGlobalStreams:  common_state_stream.DefaultMaxGlobalStreams,
	}

	router, err := NewRouter(backend,
		logger,
		flow.Testnet.Chain(),
		restCollector,
		stateStreamApi,
		stateStreamConfig.EventFilterConfig,
		stateStreamConfig.MaxGlobalStreams)
	if err != nil {
		return nil, err
	}

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr, nil
}

func assertOKResponse(t *testing.T, req *http.Request, expectedRespBody string, backend *mock.API, stateStreamApi *mock_state_stream.API) {
	assertResponse(t, req, http.StatusOK, expectedRespBody, backend, stateStreamApi)
}

func assertResponse(t *testing.T, req *http.Request, status int, expectedRespBody string, backend *mock.API, stateStreamApi *mock_state_stream.API) {
	rr, err := executeRequest(req, backend, stateStreamApi)
	assert.NoError(t, err)
	actualResponseBody := rr.Body.String()
	require.JSONEq(t,
		expectedRespBody,
		actualResponseBody,
		fmt.Sprintf("Failed Request: %s\nExpected JSON:\n %s \nActual JSON:\n %s\n", req.URL, expectedRespBody, actualResponseBody),
	)
	require.Equal(t, status, rr.Code)
}
