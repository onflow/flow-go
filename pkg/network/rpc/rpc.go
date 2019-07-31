package rpc

import (
	"context"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
)

type RPCCaller interface {
	Call(ctx context.Context, address string, method string, req proto.Message, response proto.Message) error
	Broadcast(ctx context.Context, addresses []string, method string, req proto.Message) error
}

type grpcCaller struct { }

func NewRpcCaller() RPCCaller {
	return &grpcCaller{}
}

func (caller *grpcCaller) Broadcast(ctx context.Context, addresses []string, method string, req proto.Message) error {
	var err error
	for _, addr := range addresses {
		err = caller.Call(ctx, addr, method, req, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func (caller *grpcCaller) Call(ctx context.Context, addr string, method string, req proto.Message, response proto.Message) error {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return err
	}
	err = conn.Invoke(ctx, method, req, response)
	if err != nil {
		return err
	}
	return nil
}

