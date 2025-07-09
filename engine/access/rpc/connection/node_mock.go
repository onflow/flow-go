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

func newExecutionNode(tb testing.TB) *executionNode {
	return &executionNode{
		handler: mock.NewExecutionAPIServer(tb),
	}
}

func (en *executionNode) start(tb testing.TB) {
	if en.handler == nil {
		tb.Fatalf("executionNode must be initialized using newExecutionNode")
	}

	en.setupNode(tb)
	execution.RegisterExecutionAPIServer(en.server, en.handler)
	en.node.start(tb)
}

func (en *executionNode) stop(tb testing.TB) {
	en.node.stop(tb)
}

type collectionNode struct {
	node
	handler *mock.AccessAPIServer
}

func newCollectionNode(tb testing.TB) *collectionNode {
	return &collectionNode{
		handler: mock.NewAccessAPIServer(tb),
	}
}

func (cn *collectionNode) start(tb testing.TB) {
	if cn.handler == nil {
		tb.Fatalf("collectionNode must be initialized using newCollectionNode")
	}

	cn.setupNode(tb)
	access.RegisterAccessAPIServer(cn.server, cn.handler)
	cn.node.start(tb)
}

func (cn *collectionNode) stop(tb testing.TB) {
	cn.node.stop(tb)
}
