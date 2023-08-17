package routes

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/state_stream"
	mock_state_stream "github.com/onflow/flow-go/engine/access/state_stream/mock"
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
	startBlockIdQueryParam    = "start_block_id"
	eventTypesQueryParams     = "event_types"
	addressesQueryParams      = "addresses"
	contractsQueryParams      = "contracts"
)

type fakeNetConn struct {
	io.Reader
	io.Writer
	closed chan struct{}
}

// Close closes the fakeNetConn and signals its closure by closing the "closed" channel.
func (c fakeNetConn) Close() error {
	close(c.closed)
	return nil
}
func (c fakeNetConn) LocalAddr() net.Addr                { return localAddr }
func (c fakeNetConn) RemoteAddr() net.Addr               { return remoteAddr }
func (c fakeNetConn) SetDeadline(t time.Time) error      { return nil }
func (c fakeNetConn) SetReadDeadline(t time.Time) error  { return nil }
func (c fakeNetConn) SetWriteDeadline(t time.Time) error { return nil }

type fakeAddr int

var (
	localAddr  = fakeAddr(1)
	remoteAddr = fakeAddr(2)
)

func (a fakeAddr) Network() string {
	return "net"
}

func (a fakeAddr) String() string {
	return "str"
}

// HijackResponseRecorder is a custom ResponseRecorder that implements the http.Hijacker interface
// for testing WebSocket connections and hijacking.
type HijackResponseRecorder struct {
	*httptest.ResponseRecorder
	closed       chan struct{}
	responseBuff *bytes.Buffer
}

// Hijack implements the http.Hijacker interface by returning a fakeNetConn and a bufio.ReadWriter
// that simulate a hijacked connection.
func (w *HijackResponseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	br := bufio.NewReaderSize(strings.NewReader(""), state_stream.DefaultSendBufferSize)
	bw := bufio.NewWriterSize(&bytes.Buffer{}, state_stream.DefaultSendBufferSize)
	w.responseBuff = bytes.NewBuffer(make([]byte, 0))
	w.closed = make(chan struct{}, 1)

	return fakeNetConn{strings.NewReader(""), w.responseBuff, w.closed}, bufio.NewReadWriter(br, bw), nil
}

// NewHijackResponseRecorder creates a new instance of HijackResponseRecorder.
func NewHijackResponseRecorder() *HijackResponseRecorder {
	responseRecorder := &HijackResponseRecorder{}
	responseRecorder.ResponseRecorder = httptest.NewRecorder()
	return responseRecorder
}

func newRouter(backend *mock.API, stateStreamApi *mock_state_stream.API) (*mux.Router, error) {
	var b bytes.Buffer
	logger := zerolog.New(&b)
	restCollector := metrics.NewNoopCollector()

	stateStreamConfig := state_stream.Config{
		EventFilterConfig: state_stream.DefaultEventFilterConfig,
		MaxGlobalStreams:  state_stream.DefaultMaxGlobalStreams,
	}

	return NewRouter(backend,
		logger,
		flow.Testnet.Chain(),
		restCollector,
		stateStreamApi,
		stateStreamConfig.EventFilterConfig,
		stateStreamConfig.MaxGlobalStreams)
}

func executeRequest(req *http.Request, backend *mock.API, stateStreamApi *mock_state_stream.API) (*HijackResponseRecorder, error) {
	router, err := newRouter(backend, stateStreamApi)
	if err != nil {
		return nil, err
	}

	resp := NewHijackResponseRecorder()
	router.ServeHTTP(resp, req)
	return resp, nil
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
