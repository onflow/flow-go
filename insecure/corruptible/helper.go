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

func (m *mockAttackerObserveClient) SetHeader(_ metadata.MD) error {
	panic("not implemented")
}

func (m *mockAttackerObserveClient) SendHeader(_ metadata.MD) error {
	panic("not implemented")
}

func (m *mockAttackerObserveClient) SetTrailer(_ metadata.MD) {
	panic("not implemented")
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
