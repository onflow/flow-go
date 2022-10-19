package attackernet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/onflow/flow-go/module"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

const networkingProtocolTCP = "tcp"

// mockCorruptNetwork is a mock corrupt network that implements only the side that interacts with the attacker network
// (running a gRPC server, and accepting gRPC streams from attacker network).
// However, instead of dispatching incoming messages to the networking layer, the
// mock corrupt network only keeps the incoming message in a channel. The purpose of this mock corrupt network is to empower tests evaluating the interactions
// between the attacker network and the gRPC side of corrupt network.
type mockCorruptNetwork struct {
	component.Component
	cm                    *component.ComponentManager
	server                *grpc.Server           // touch point of attacker network to this factory.
	address               net.Addr               // address of gRPC endpoint for this mock corrupt network
	attackerRegMsg        chan interface{}       // channel indicating whether attacker has registered.
	attackerMsg           chan *insecure.Message // channel  keeping the last incoming (insecure) message from an attacker.
	attackerObserveStream insecure.CorruptNetwork_ConnectAttackerServer
}

var _ insecure.CorruptNetworkServer = &mockCorruptNetwork{}
var _ module.ReadyDoneAware = &mockCorruptNetwork{}
var _ module.Startable = &mockCorruptNetwork{}

func newMockCorruptNetwork() *mockCorruptNetwork {
	factory := &mockCorruptNetwork{
		attackerRegMsg: make(chan interface{}),
		attackerMsg:    make(chan *insecure.Message),
	}

	cm := component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			factory.start(ctx, insecure.DefaultAddress)

			ready()

			<-ctx.Done()
			factory.stop()
		}).Build()

	factory.Component = cm
	factory.cm = cm

	return factory
}

func (c *mockCorruptNetwork) ServerAddress() string {
	return c.address.String()
}

func (c *mockCorruptNetwork) start(ctx irrecoverable.SignalerContext, address string) {
	// starts up gRPC server of corrupt network at given address.
	s := grpc.NewServer()
	insecure.RegisterCorruptNetworkServer(s, c)
	ln, err := net.Listen(networkingProtocolTCP, address)
	if err != nil {
		ctx.Throw(fmt.Errorf("could not listen on specified address: %w", err))
	}
	c.server = s
	c.address = ln.Addr()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		wg.Done()
		if err = s.Serve(ln); err != nil { // blocking call
			ctx.Throw(fmt.Errorf("could not bind factory to the tcp listener: %w", err))
		}
	}()

	wg.Wait()
}

func (c *mockCorruptNetwork) stop() {
	c.server.Stop()
}

// ProcessAttackerMessage is a gRPC end-point of this mock corrupt network, that accepts messages from attacker network, and
// puts the incoming message in a channel to be read by the test procedure.
func (c *mockCorruptNetwork) ProcessAttackerMessage(stream insecure.CorruptNetwork_ProcessAttackerMessageServer) error {
	for {
		select {
		case <-c.cm.ShutdownSignal():
			return nil
		default:
			msg, err := stream.Recv()
			if err == io.EOF || errors.Is(stream.Context().Err(), context.Canceled) {
				return stream.SendAndClose(&empty.Empty{})
			}
			if err != nil {
				return stream.SendAndClose(&empty.Empty{})
			}
			c.attackerMsg <- msg
		}
	}
}

// ConnectAttacker is a gRPC end-point of this mock corrupt network, that accepts attacker registration messages from the attacker network.
// It puts the incoming message into a channel to be read by test procedure.
func (c *mockCorruptNetwork) ConnectAttacker(_ *empty.Empty, stream insecure.CorruptNetwork_ConnectAttackerServer) error {
	select {
	case <-c.cm.ShutdownSignal():
		return nil

	default:
		c.attackerObserveStream = stream
		close(c.attackerRegMsg)
	}

	// WARNING: this method call should not return through the entire lifetime of this corrupt network.
	// This is a client streaming gRPC implementation, and the input stream's lifecycle
	// is tightly coupled with the lifecycle of this function call.
	// Once it returns, the client stream is closed forever.
	// Hence, we block the call and wait till a component shutdown.
	<-c.cm.ShutdownSignal()

	return nil
}

// withMockCorruptNetwork creates and starts a mock corrupt network. This mock corrupt network only runs the gRPC server part of an
// actual corrupt network, and then executes the run function on it.
func withMockCorruptNetwork(t *testing.T, run func(flow.Identity, irrecoverable.SignalerContext, *mockCorruptNetwork)) {
	corruptIdentity := unittest.IdentityFixture(unittest.WithAddress(insecure.DefaultAddress))

	// life-cycle management of corrupt network.
	ctx, cancel := context.WithCancel(context.Background())
	cnCtx, errChan := irrecoverable.WithSignaler(ctx)
	go func() {
		select {
		case err := <-errChan:
			t.Error("mock corrupt network startup encountered fatal error", err)
		case <-ctx.Done():
			return
		}
	}()

	cn := newMockCorruptNetwork()

	// starts corrupt network
	cn.Start(cnCtx)
	unittest.RequireCloseBefore(t, cn.Ready(), 100*time.Millisecond, "could not start corrupt network on time")

	run(*corruptIdentity, cnCtx, cn)

	// terminates corrupt network
	cancel()
	unittest.RequireCloseBefore(t, cn.Done(), 100*time.Millisecond, "could not stop corrupt network on time")
}
