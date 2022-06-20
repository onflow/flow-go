package corruptible

import (
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/onflow/flow-go/insecure"
)

type mockAttackerObserveClient struct {
	grpc.ClientStream
	incomingBuffer chan *insecure.Message
	closed         chan interface{}
}

func newMockAttackerObserverClient() *mockAttackerObserveClient {
	return &mockAttackerObserveClient{
		incomingBuffer: make(chan *insecure.Message),
		closed:         make(chan interface{}),
	}
}

// mockAttackerObserveClient must implement CorruptibleConduitFactory_ProcessAttackerMessageClient interface.
var _ insecure.CorruptibleConduitFactory_ProcessAttackerMessageClient = &mockAttackerObserveClient{}

// SetHeader is only added to satisfy the interface implementation.
func (m *mockAttackerObserveClient) SetHeader(_ metadata.MD) error {
	panic("not implemented")
}

// SendHeader is only added to satisfy the interface implementation.
func (m *mockAttackerObserveClient) SendHeader(_ metadata.MD) error {
	panic("not implemented")
}

// SetTrailer is only added to satisfy the interface implementation.
func (m *mockAttackerObserveClient) SetTrailer(_ metadata.MD) {
	panic("not implemented")
}

// Send puts the incoming message into inbound buffer of the attacker.
func (m *mockAttackerObserveClient) Send(in *insecure.Message) error {
	m.incomingBuffer <- in
	return nil
}

// CloseAndRecv notifies the attacker that the client is closed.
func (m *mockAttackerObserveClient) CloseAndRecv() (*empty.Empty, error) {
	close(m.closed)
	return &empty.Empty{}, nil
}
