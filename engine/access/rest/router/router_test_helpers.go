package router

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
	"github.com/onflow/flow-go/access/backends/extended"
	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	"github.com/onflow/flow-go/engine/access/subscription"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

const (
	ExpandableFieldPayload      = "payload"
	ExpandableExecutionResult   = "execution_result"
	SealedHeightQueryParam      = "sealed"
	FinalHeightQueryParam       = "final"
	StartHeightQueryParam       = "start_height"
	EndHeightQueryParam         = "end_height"
	HeightQueryParam            = "height"
	StartBlockIdQueryParam      = "start_block_id"
	EventTypesQueryParams       = "event_types"
	AddressesQueryParams        = "addresses"
	ContractsQueryParams        = "contracts"
	HeartbeatIntervalQueryParam = "heartbeat_interval"
)

// fakeNetConn implements a mocked ws connection that can be injected in testing logic.
type fakeNetConn struct {
	io.Writer
	closed chan struct{}
}

var _ net.Conn = (*fakeNetConn)(nil)

// Close closes the fakeNetConn and signals its closure by closing the "Closed" channel.
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

// TestHijackResponseRecorder is a custom ResponseRecorder that implements the http.Hijacker interface
// for testing WebSocket connections and hijacking.
type TestHijackResponseRecorder struct {
	*httptest.ResponseRecorder
	Closed       chan struct{}
	ResponseBuff *bytes.Buffer
}

var _ http.Hijacker = (*TestHijackResponseRecorder)(nil)

// Hijack implements the http.Hijacker interface by returning a fakeNetConn and a bufio.ReadWriter
// that simulate a hijacked connection.
func (w *TestHijackResponseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	br := bufio.NewReaderSize(strings.NewReader(""), subscription.DefaultSendBufferSize)
	bw := bufio.NewWriterSize(&bytes.Buffer{}, subscription.DefaultSendBufferSize)
	w.ResponseBuff = bytes.NewBuffer(make([]byte, 0))
	w.Closed = make(chan struct{}, 1)

	return fakeNetConn{w.ResponseBuff, w.Closed}, bufio.NewReadWriter(br, bw), nil
}

func (w *TestHijackResponseRecorder) Close() error {
	select {
	case <-w.Closed:
	default:
		close(w.Closed)
	}
	return nil
}

// NewTestHijackResponseRecorder creates a new instance of TestHijackResponseRecorder.
func NewTestHijackResponseRecorder() *TestHijackResponseRecorder {
	return &TestHijackResponseRecorder{
		ResponseRecorder: httptest.NewRecorder(),
	}
}

func ExecuteRequest(req *http.Request, backend access.API) *httptest.ResponseRecorder {
	router := NewRouterBuilder(
		unittest.Logger(),
		metrics.NewNoopCollector(),
	).AddRestRoutes(
		backend,
		flow.Testnet.Chain(),
		commonrpc.DefaultAccessMaxRequestSize,
		commonrpc.DefaultAccessMaxResponseSize,
	).Build()

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func ExecuteLegacyWsRequest(req *http.Request, stateStreamApi state_stream.API, responseRecorder *TestHijackResponseRecorder, chain flow.Chain) {
	restCollector := metrics.NewNoopCollector()

	config := backend.Config{
		EventFilterConfig: state_stream.DefaultEventFilterConfig,
		MaxGlobalStreams:  subscription.DefaultMaxGlobalStreams,
		HeartbeatInterval: subscription.DefaultHeartbeatInterval,
	}

	router := NewRouterBuilder(
		unittest.Logger(),
		restCollector,
	).AddLegacyWebsocketsRoutes(
		stateStreamApi,
		chain, config, commonrpc.DefaultAccessMaxRequestSize, commonrpc.DefaultAccessMaxResponseSize,
	).Build()
	router.ServeHTTP(responseRecorder, req)
}

func AssertOKResponse(t *testing.T, req *http.Request, expectedRespBody string, backend *mock.API) {
	AssertResponse(t, req, http.StatusOK, expectedRespBody, backend)
}

// ExecuteExperimentalRequest builds a router with experimental routes and executes the given request.
func ExecuteExperimentalRequest(req *http.Request, backend extended.API) *httptest.ResponseRecorder {
	router := NewRouterBuilder(
		unittest.Logger(),
		metrics.NewNoopCollector(),
	).AddExperimentalRoutes(
		backend,
		flow.Testnet.Chain(),
		commonrpc.DefaultAccessMaxRequestSize,
		commonrpc.DefaultAccessMaxResponseSize,
	).Build()

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func AssertResponse(t *testing.T, req *http.Request, status int, expectedRespBody string, backend *mock.API) {
	rr := ExecuteRequest(req, backend)
	actualResponseBody := rr.Body.String()
	require.JSONEq(t,
		expectedRespBody,
		actualResponseBody,
		fmt.Sprintf("Failed Request: %s\nExpected JSON:\n %s \nActual JSON:\n %s\n", req.URL, expectedRespBody, actualResponseBody),
	)
	require.Equal(t, status, rr.Code)
}
