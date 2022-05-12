package corruptible

import (
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

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

func (m *mockAttackerObserveClient) Send(in *insecure.Message) error {
	m.incomingBuffer <- in
	return nil
}

func (m *mockAttackerObserveClient) CloseAndRecv() (*empty.Empty, error) {
	close(m.closed)
	return &empty.Empty{}, nil
}
