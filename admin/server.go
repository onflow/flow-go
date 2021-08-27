package admin

import (
	"context"

	pb "github.com/onflow/flow-go/admin/admin"
	"google.golang.org/protobuf/types/known/structpb"
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
	err  error
	data map[string]interface{}
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
		return nil, ctx.Err()
	}

	// TODO: also use a select around ctx.done() here?
	response, ok := <-resp
	if !ok {
		// TODO: closed without response
		// this means that there was no handler
		// so return a sentinel error ErrUnexpectedCommand
	}

	if response.err != nil {
		// TODO: define sentinel error here
		return nil, response.err
	}

	data, err := structpb.NewStruct(response.data)
	if err != nil {
		// TODO: probably should log fatal here,
		// this means the handler returned invalid
		// data
	}

	return &pb.RunCommandResponse{
		Data: data,
	}, nil
}

func NewAdminServer(commandQ chan<- *CommandRequest) *adminServer {
	return &adminServer{commandQ: commandQ}
}
