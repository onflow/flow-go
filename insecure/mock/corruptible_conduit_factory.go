package mockinsecure

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

const listenerBufferSize = 1024 * 1024

type CorruptibleConduitFactory struct {
	server *grpc.Server
	*component.ComponentManager
	t                       *testing.T
	attackerRegisterHandler func(*testing.T, *insecure.AttackerRegisterMessage)
	processAttackMessage    func(*testing.T, *insecure.Message)
}

func (c *CorruptibleConduitFactory) RegisterAttacker(_ context.Context, message *insecure.AttackerRegisterMessage) (*empty.Empty, error) {
	c.attackerRegisterHandler(c.t, message)
	return &empty.Empty{}, nil
}

func (c *CorruptibleConduitFactory) ProcessAttackerMessage(stream insecure.CorruptibleConduitFactory_ProcessAttackerMessageServer) error {
	for {
		select {
		case <-c.ComponentManager.ShutdownSignal():
			return nil
		default:
			msg, err := stream.Recv()
			if err == io.EOF {
				return stream.SendAndClose(&empty.Empty{})
			}
			require.NoError(c.t, err)
			c.processAttackMessage(c.t, msg)
		}
	}
}

func NewCorruptibleConduitFactory(t *testing.T,
	attackRegisterHandler func(*testing.T, *insecure.AttackerRegisterMessage),
	processAttackMessage func(*testing.T, *insecure.Message)) *CorruptibleConduitFactory {

	c := &CorruptibleConduitFactory{
		attackerRegisterHandler: attackRegisterHandler,
		processAttackMessage:    processAttackMessage,
		t:                       t,
	}

	// setting lifecycle management module.
	cm := component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			s := grpc.NewServer()
			insecure.RegisterCorruptibleConduitFactoryServer(s, c)
			ln := bufconn.Listen(listenerBufferSize)
			c.server = s

			wg := sync.WaitGroup{}
			wg.Add(1)
			go func() {
				wg.Done()
				if err := s.Serve(ln); err != nil {
					ctx.Throw(fmt.Errorf("could not bind mock factory to listener: %w", err))
				}
			}()

			wg.Wait()
			ready()
		}).Build()

	c.ComponentManager = cm

	return c
}
