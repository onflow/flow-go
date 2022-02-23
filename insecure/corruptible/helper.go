package corruptible

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/insecure"
)

type mockAttacker struct {
	incomingBuffer chan *insecure.Message
}

func (m *mockAttacker) Observe(_ context.Context, in *insecure.Message, _ ...grpc.CallOption) (*empty.Empty, error) {
	m.incomingBuffer <- in
	return &empty.Empty{}, nil
}
