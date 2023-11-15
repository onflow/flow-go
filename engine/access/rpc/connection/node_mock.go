package connection

import (
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// node mocks a flow node that runs a GRPC server
type node struct {
	server   *grpc.Server
	listener net.Listener
	port     uint
}

func (n *node) setupNode(tb testing.TB) {
	n.server = grpc.NewServer()
	listener, err := net.Listen("tcp4", unittest.DefaultAddress)
	assert.NoError(tb, err)
	n.listener = listener
	assert.Eventually(tb, func() bool {
		return !strings.HasSuffix(listener.Addr().String(), ":0")
	}, time.Second*4, 10*time.Millisecond)

	_, port, err := net.SplitHostPort(listener.Addr().String())
	assert.NoError(tb, err)
	portAsUint, err := strconv.ParseUint(port, 10, 32)
	assert.NoError(tb, err)
	n.port = uint(portAsUint)
}

func (n *node) start(tb testing.TB) {
	// using a wait group here to ensure the goroutine has started before returning. Otherwise,
	// there's a race condition where the server is sometimes stopped before it has started
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		wg.Done()
		err := n.server.Serve(n.listener)
		assert.NoError(tb, err)
	}()
	unittest.RequireReturnsBefore(tb, wg.Wait, 10*time.Millisecond, "could not start goroutine on time")
}

func (n *node) stop(tb testing.TB) {
	if n.server != nil {
		n.server.Stop()
	}
}

type executionNode struct {
	node
	handler *mock.ExecutionAPIServer
}

func (en *executionNode) start(tb testing.TB) {
	en.setupNode(tb)
	handler := new(mock.ExecutionAPIServer)
	execution.RegisterExecutionAPIServer(en.server, handler)
	en.handler = handler
	en.node.start(tb)
}

func (en *executionNode) stop(tb testing.TB) {
	en.node.stop(tb)
}

type collectionNode struct {
	node
	handler *mock.AccessAPIServer
}

func (cn *collectionNode) start(t *testing.T) {
	cn.setupNode(t)
	handler := new(mock.AccessAPIServer)
	access.RegisterAccessAPIServer(cn.server, handler)
	cn.handler = handler
	cn.node.start(t)
}

func (cn *collectionNode) stop(t *testing.T) {
	cn.node.stop(t)
}
