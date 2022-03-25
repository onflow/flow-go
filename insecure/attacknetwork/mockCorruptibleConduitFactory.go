package attacknetwork

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// mockCorruptibleConduitFactory is a mock corruptible conduit factory (ccf) that implements only the side that interacts with the attack network,
// i.e., running a gRPC server, and accepting gRPC streams from attack network. However,
// instead of dispatching incoming messages to the networking layer, the
// mock ccf only keeps the incoming message in a channel. The purpose of this mock ccf is to empower tests evaluating the interactions
// between the attack network and the gRPC side of ccf.
type mockCorruptibleConduitFactory struct {
	component.Component
	cm                    *component.ComponentManager
	attackerObserveClient insecure.Attacker_ObserveClient
	server                *grpc.Server // touch point of attack network to this factory.
	address               net.Addr
	attackerRegMsg        chan *insecure.AttackerRegisterMessage
	attackerMsg           chan *insecure.Message
}

func newMockCorruptibleConduitFactory() *mockCorruptibleConduitFactory {

	factory := &mockCorruptibleConduitFactory{
		attackerRegMsg: make(chan *insecure.AttackerRegisterMessage),
		attackerMsg:    make(chan *insecure.Message),
	}

	cm := component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			factory.start(ctx, "localhost:0")

			ready()

			<-ctx.Done()

			factory.stop()
		}).Build()

	factory.Component = cm
	factory.cm = cm

	return factory
}

func (c *mockCorruptibleConduitFactory) ServerAddress() string {
	return c.address.String()
}

func (c *mockCorruptibleConduitFactory) start(ctx irrecoverable.SignalerContext, address string) {
	// starts up gRPC server of attack network at given address.
	s := grpc.NewServer()
	insecure.RegisterCorruptibleConduitFactoryServer(s, c)
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

func (c *mockCorruptibleConduitFactory) stop() {
	c.server.Stop()
}

// ProcessAttackerMessage is a gRPC end-point of this mock corruptible conduit factory, that accepts messages from attack network, and
// puts the incoming message in a channel to be read by the test procedure.
func (c *mockCorruptibleConduitFactory) ProcessAttackerMessage(stream insecure.CorruptibleConduitFactory_ProcessAttackerMessageServer) error {
	for {
		select {
		case <-c.cm.ShutdownSignal():
			if c.attackerObserveClient != nil {
				_, err := c.attackerObserveClient.CloseAndRecv()
				if err != nil {
					return err
				}
			}
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

// RegisterAttacker is a gRPC end-point of this mock corruptible conduit factory, that accepts attacker registration messages from the attack network.
// It puts the incoming message into a channel to be read by test procedure.
func (c *mockCorruptibleConduitFactory) RegisterAttacker(_ context.Context, in *insecure.AttackerRegisterMessage) (*empty.Empty, error) {
	select {
	case <-c.cm.ShutdownSignal():
		return nil, fmt.Errorf("conduit factory has been shut down")
	default:
		c.attackerRegMsg <- in
		return &empty.Empty{}, nil
	}
}
