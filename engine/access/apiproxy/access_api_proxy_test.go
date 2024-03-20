package apiproxy

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/grpc/forwarder"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/grpcutils"
	"github.com/onflow/flow-go/utils/unittest"
)

// Methodology
//
// We test the proxy and fall-over logic to reach basic coverage.
//
// * Basic coverage means that all conditional checks happen once but only once.
// * We embrace the simplest adequate solution to reduce engineering cost.
// * Any use cases requiring multiple conditionals exercised in a row are considered ignorable due to cost constraints.

// TestNetE2E tests the basic unix network first
func TestNetE2E(t *testing.T) {
	done := make(chan int)
	// Bring up 1st upstream server
	charlie1, err := makeFlowLite("/tmp/TestProxyE2E1", done)
	if err != nil {
		t.Fatal(err)
	}
	// Wait until proxy call passes
	err = callFlowLite("/tmp/TestProxyE2E1")
	if err != nil {
		t.Fatal(err)
	}
	// Bring up 2nd upstream server
	charlie2, err := makeFlowLite("/tmp/TestProxyE2E2", done)
	if err != nil {
		t.Fatal(err)
	}
	// Both proxy calls should pass
	err = callFlowLite("/tmp/TestProxyE2E1")
	if err != nil {
		t.Fatal(err)
	}
	err = callFlowLite("/tmp/TestProxyE2E2")
	if err != nil {
		t.Fatal(err)
	}
	// Stop 1st upstream server
	_ = charlie1.Close()
	// Proxy call falls through
	err = callFlowLite("/tmp/TestProxyE2E1")
	if err == nil {
		t.Fatal(fmt.Errorf("backend still active after close"))
	}
	// Stop 2nd upstream server
	_ = charlie2.Close()
	// System errors out on shut down servers
	err = callFlowLite("/tmp/TestProxyE2E1")
	if err == nil {
		t.Fatal(fmt.Errorf("backend still active after close"))
	}
	err = callFlowLite("/tmp/TestProxyE2E2")
	if err == nil {
		t.Fatal(fmt.Errorf("backend still active after close"))
	}
	// wait for all
	<-done
	<-done
}

// TestgRPCE2E tests whether gRPC works
func TestGRPCE2E(t *testing.T) {
	done := make(chan int)
	// Bring up 1st upstream server
	charlie1, _, err := newFlowLite("unix", "/tmp/TestProxyE2E1", done)
	if err != nil {
		t.Fatal(err)
	}
	// Wait until proxy call passes
	err = openFlowLite("/tmp/TestProxyE2E1")
	if err != nil {
		t.Fatal(err)
	}
	// Bring up 2nd upstream server
	charlie2, _, err := newFlowLite("unix", "/tmp/TestProxyE2E2", done)
	if err != nil {
		t.Fatal(err)
	}
	// Both proxy calls should pass
	err = openFlowLite("/tmp/TestProxyE2E1")
	if err != nil {
		t.Fatal(err)
	}
	err = openFlowLite("/tmp/TestProxyE2E2")
	if err != nil {
		t.Fatal(err)
	}
	// Stop 1st upstream server
	charlie1.Stop()
	// Proxy call falls through
	err = openFlowLite("/tmp/TestProxyE2E1")
	if err == nil {
		t.Fatal(fmt.Errorf("backend still active after close"))
	}
	// Stop 2nd upstream server
	charlie2.Stop()
	// System errors out on shut down servers
	err = openFlowLite("/tmp/TestProxyE2E1")
	if err == nil {
		t.Fatal(fmt.Errorf("backend still active after close"))
	}
	err = openFlowLite("/tmp/TestProxyE2E2")
	if err == nil {
		t.Fatal(fmt.Errorf("backend still active after close"))
	}
	// wait for all
	<-done
	<-done
}

// TestNewFlowCachedAccessAPIProxy tests the round robin end to end
func TestNewFlowCachedAccessAPIProxy(t *testing.T) {
	done := make(chan int)

	// Bring up 1st upstream server
	charlie1, _, err := newFlowLite("tcp", unittest.IPPort("11634"), done)
	if err != nil {
		t.Fatal(err)
	}

	// Bring up 2nd upstream server
	charlie2, _, err := newFlowLite("tcp", unittest.IPPort("11635"), done)
	if err != nil {
		t.Fatal(err)
	}

	metrics := metrics.NewNoopCollector()

	// create the factory
	connectionFactory := &connection.ConnectionFactoryImpl{
		// set metrics reporting
		AccessMetrics:             metrics,
		CollectionNodeGRPCTimeout: time.Second,
		Manager: connection.NewManager(
			unittest.Logger(),
			metrics,
			nil,
			grpcutils.DefaultMaxMsgSize,
			connection.CircuitBreakerConfig{},
			grpcutils.NoCompressor,
		),
	}

	// Prepare a proxy that fails due to the second connection being idle
	l := flow.IdentitySkeletonList{{Address: unittest.IPPort("11634")}, {Address: unittest.IPPort("11635")}}
	c := FlowAccessAPIForwarder{}
	c.Forwarder, err = forwarder.NewForwarder(l, connectionFactory)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Wait until proxy call passes
	_, err = c.Ping(ctx, &access.PingRequest{})
	if err != nil {
		t.Fatal(err)
	}

	// get and close first connection
	_, closer, err := c.Forwarder.FaultTolerantClient()
	assert.NoError(t, err)
	closer.Close()

	// connection factory created a new gRPC connection which was closed before
	// if creation fails should use second connection
	// Wait until proxy call passes
	_, err = c.Ping(ctx, &access.PingRequest{})
	if err != nil {
		t.Fatal(err)
	}

	// Wait until proxy call passes
	_, err = c.Ping(ctx, &access.PingRequest{})
	if err != nil {
		t.Fatal(err)
	}

	charlie1.Stop()
	charlie2.Stop()

	// Wait until proxy call fails
	_, err = c.Ping(ctx, &access.PingRequest{})
	if err == nil {
		t.Fatal(fmt.Errorf("should fail on no connections"))
	}

	<-done
	<-done
}

func makeFlowLite(address string, done chan int) (net.Listener, error) {
	l, err := net.Listen("unix", address)
	if err != nil {
		return nil, err
	}

	go func(done chan int) {
		for {
			c, err := l.Accept()
			if err != nil {
				break
			}

			b := make([]byte, 3)
			_, _ = c.Read(b)
			_, _ = c.Write(b)
			_ = c.Close()
		}
		done <- 1
	}(done)
	return l, err
}

func callFlowLite(address string) error {
	c, err := net.Dial("unix", address)
	if err != nil {
		return err
	}
	o := []byte("abc")
	_, _ = c.Write(o)
	i := make([]byte, 3)
	_, _ = c.Read(i)
	if string(o) != string(i) {
		return fmt.Errorf("no match")
	}
	_ = c.Close()
	_ = MockFlowAccessAPI{}
	return err
}

func newFlowLite(network string, address string, done chan int) (*grpc.Server, *net.Listener, error) {
	l, err := net.Listen(network, address)
	if err != nil {
		return nil, nil, err
	}
	s := grpc.NewServer()
	go func(done chan int) {
		access.RegisterAccessAPIServer(s, MockFlowAccessAPI{})
		_ = s.Serve(l)
		done <- 1
	}(done)
	return s, &l, nil
}

func openFlowLite(address string) error {
	c, err := grpc.Dial(
		"unix://"+address,
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()))
	if err != nil {
		return err
	}
	a := access.NewAccessAPIClient(c)

	background := context.Background()

	_, err = a.Ping(background, &access.PingRequest{})
	if err != nil {
		return err
	}

	return nil
}

type MockFlowAccessAPI struct {
	access.AccessAPIServer
}

// Ping is used to check if the access node is alive and healthy.
func (p MockFlowAccessAPI) Ping(context.Context, *access.PingRequest) (*access.PingResponse, error) {
	return &access.PingResponse{}, nil
}
