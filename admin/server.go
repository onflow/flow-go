package admin

import (
	"context"
	"fmt"

	pb "github.com/onflow/flow-go/admin/admin"
)

type adminServer struct {
	pb.UnimplementedAdminServer
	cr *CommandRunner
}

func (s *adminServer) RunCommand(ctx context.Context, in *pb.RunCommandRequest) (*pb.RunCommandResponse, error) {
	var result interface{}
	var err error

	if result, err = s.cr.runCommand(ctx, in.GetCommandName(), in.GetData().AsMap()); err != nil {
		return nil, err
	}

	return &pb.RunCommandResponse{
		Output: fmt.Sprintf("%v", result),
	}, nil
}

func NewAdminServer(cr *CommandRunner) *adminServer {
	return &adminServer{cr: cr}
}
