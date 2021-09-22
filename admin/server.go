package admin

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/onflow/flow-go/admin/admin"
)

type adminServer struct {
	pb.UnimplementedAdminServer
	commandQ chan<- *CommandRequest
}

type CommandRequest struct {
	ctx          context.Context
	command      string
	data         map[string]interface{}
	responseChan chan<- *CommandResponse
}

type CommandResponse struct {
	err error
}

func (s *adminServer) RunCommand(ctx context.Context, in *pb.RunCommandRequest) (*pb.RunCommandResponse, error) {
	resp := make(chan *CommandResponse, 1)

	select {
	case s.commandQ <- &CommandRequest{
		ctx,
		in.GetCommandName(),
		in.GetData().AsMap(),
		resp,
	}:
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, status.Error(codes.Canceled, "client canceled")
		} else if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, status.Error(codes.DeadlineExceeded, "request timed out")
		}

		panic("context returned unexpected error after done channel was closed")
	}

	response, ok := <-resp
	if !ok {
		// response channel was closed without a response
		return nil, status.Error(codes.Internal, "command terminated unexpectedly")
	}

	if response.err != nil {
		return nil, response.err
	}

	return &pb.RunCommandResponse{}, nil
}

func NewAdminServer(commandQ chan<- *CommandRequest) *adminServer {
	return &adminServer{commandQ: commandQ}
}
