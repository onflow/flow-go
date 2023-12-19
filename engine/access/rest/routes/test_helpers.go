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

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

const (
	ExpandableFieldPayload      = "payload"
	ExpandableExecutionResult   = "execution_result"
	sealedHeightQueryParam      = "sealed"
	finalHeightQueryParam       = "final"
	startHeightQueryParam       = "start_height"
	endHeightQueryParam         = "end_height"
	heightQueryParam            = "height"
	startBlockIdQueryParam      = "start_block_id"
	eventTypesQueryParams       = "event_types"
	addressesQueryParams        = "addresses"
	contractsQueryParams        = "contracts"
	heartbeatIntervalQueryParam = "heartbeat_interval"
)

// fakeNetConn implements a mocked ws connection that can be injected in testing logic.
type fakeNetConn struct {
	io.Writer
	closed chan struct{}
}

var _ net.Conn = (*fakeNetConn)(nil)

// Close closes the fakeNetConn and signals its closure by closing the "closed" channel.
func (c fakeNetConn) Close() error {
	select {
	case <-c.closed:
	default:
		close(c.closed)
	}
	return nil
}
func (c fakeNetConn) LocalAddr() net.Addr                { return localAddr }
func (c fakeNetConn) RemoteAddr() net.Addr               { return remoteAddr }
func (c fakeNetConn) SetDeadline(t time.Time) error      { return nil }
func (c fakeNetConn) SetReadDeadline(t time.Time) error  { return nil }
func (c fakeNetConn) SetWriteDeadline(t time.Time) error { return nil }
func (c fakeNetConn) Read(p []byte) (n int, err error) {
	<-c.closed
	return 0, fmt.Errorf("closed")
}

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

// testHijackResponseRecorder is a custom ResponseRecorder that implements the http.Hijacker interface
// for testing WebSocket connections and hijacking.
type testHijackResponseRecorder struct {
	*httptest.ResponseRecorder
	closed       chan struct{}
	responseBuff *bytes.Buffer
}

var _ http.Hijacker = (*testHijackResponseRecorder)(nil)

// Hijack implements the http.Hijacker interface by returning a fakeNetConn and a bufio.ReadWriter
// that simulate a hijacked connection.
func (w *testHijackResponseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	br := bufio.NewReaderSize(strings.NewReader(""), subscription.DefaultSendBufferSize)
	bw := bufio.NewWriterSize(&bytes.Buffer{}, subscription.DefaultSendBufferSize)
	w.responseBuff = bytes.NewBuffer(make([]byte, 0))
	w.closed = make(chan struct{}, 1)

	return fakeNetConn{w.responseBuff, w.closed}, bufio.NewReadWriter(br, bw), nil
}

// newTestHijackResponseRecorder creates a new instance of testHijackResponseRecorder.
func newTestHijackResponseRecorder() *testHijackResponseRecorder {
	return &testHijackResponseRecorder{
		ResponseRecorder: httptest.NewRecorder(),
	}
}

func executeRequest(req *http.Request, backend access.API) *httptest.ResponseRecorder {
	router := NewRouterBuilder(
		unittest.Logger(),
		metrics.NewNoopCollector(),
	).AddRestRoutes(
		backend,
		flow.Testnet.Chain(),
	).Build()

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func executeWsRequest(req *http.Request, stateStreamApi state_stream.API, responseRecorder *testHijackResponseRecorder) {
	restCollector := metrics.NewNoopCollector()

	config := backend.Config{
		EventFilterConfig: state_stream.DefaultEventFilterConfig,
		MaxGlobalStreams:  subscription.DefaultMaxGlobalStreams,
		HeartbeatInterval: subscription.DefaultHeartbeatInterval,
	}

	router := NewRouterBuilder(unittest.Logger(), restCollector).AddWsRoutes(
		stateStreamApi,
		flow.Testnet.Chain(), config).Build()
	router.ServeHTTP(responseRecorder, req)
}

func assertOKResponse(t *testing.T, req *http.Request, expectedRespBody string, backend *mock.API) {
	assertResponse(t, req, http.StatusOK, expectedRespBody, backend)
}

func assertResponse(t *testing.T, req *http.Request, status int, expectedRespBody string, backend *mock.API) {
	rr := executeRequest(req, backend)
	actualResponseBody := rr.Body.String()
	require.JSONEq(t,
		expectedRespBody,
		actualResponseBody,
		fmt.Sprintf("Failed Request: %s\nExpected JSON:\n %s \nActual JSON:\n %s\n", req.URL, expectedRespBody, actualResponseBody),
	)
	require.Equal(t, status, rr.Code)
}
