package admin

import (
	"context"

	pb "github.com/onflow/flow-go/admin/admin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

type adminServer struct {
	pb.UnimplementedAdminServer
	cr *CommandRunner
}

func (s *adminServer) RunCommand(ctx context.Context, in *pb.RunCommandRequest) (*pb.RunCommandResponse, error) {
	result, err := s.cr.runCommand(ctx, in.GetCommandName(), in.GetData().AsInterface())
	if err != nil {
		return nil, err
	}

	value, err := structpb.NewValue(result)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.RunCommandResponse{
		Output: value,
	}, nil
}

func NewAdminServer(cr *CommandRunner) *adminServer {
	return &adminServer{cr: cr}
}
