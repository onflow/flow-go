package corruptnet

import (
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/onflow/flow-go/insecure"
)

// mockAttacker is used for unit testing the corrupt network by abstracting away the gRPC implementation
// It doesn't have any networking primitive, so we just can test the interaction between the corrupt network and the attacker observer client.
type mockAttacker struct {
	grpc.ClientStream

	// incomingBuffer imitates the gRPC buffer of the orchestrator network,
	// i.e., when a corrupt network is relaying an ingress/egress message to an attacker for observation,
	// the message goes to the gRPC buffer (i.e., incomingBuffer).
	incomingBuffer chan *insecure.Message
	closed         chan interface{}
}

func newMockAttacker() *mockAttacker {
	return &mockAttacker{
		incomingBuffer: make(chan *insecure.Message),
		closed:         make(chan interface{}),
	}
}

// mockAttacker must implement CorruptibleConduitFactory_ProcessAttackerMessageClient interface.
var _ insecure.CorruptNetwork_ProcessAttackerMessageClient = &mockAttacker{}

// SetHeader is only added to satisfy the interface implementation.
func (m *mockAttacker) SetHeader(_ metadata.MD) error {
	panic("not implemented")
}

// SendHeader is only added to satisfy the interface implementation.
func (m *mockAttacker) SendHeader(_ metadata.MD) error {
	panic("not implemented")
}

// SetTrailer is only added to satisfy the interface implementation.
func (m *mockAttacker) SetTrailer(_ metadata.MD) {
	panic("not implemented")
}

// Send puts the incoming message into inbound buffer of the attacker.
func (m *mockAttacker) Send(in *insecure.Message) error {
	m.incomingBuffer <- in
	return nil
}

// CloseAndRecv notifies the attacker that the client is closed.
func (m *mockAttacker) CloseAndRecv() (*empty.Empty, error) {
	close(m.closed)
	return &empty.Empty{}, nil
}
