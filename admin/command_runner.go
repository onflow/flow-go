package admin

import (
	"context"
	"fmt"
	"net"
	"sync"

	"google.golang.org/grpc"

	pb "github.com/onflow/flow-go/admin/admin"
)

const (
	CommandRunnerMaxQueueLength = 128
)

type CommandRunner struct {
	mu       sync.RWMutex
	handlers map[string]CommandHandler
	commandQ chan *CommandRequest
}

type CommandHandler func(ctx context.Context, data map[string]interface{}) error

func NewCommandRunner() *CommandRunner {
	return &CommandRunner{
		handlers: make(map[string]CommandHandler),
		commandQ: make(chan *CommandRequest, CommandRunnerMaxQueueLength),
	}
}

func (r *CommandRunner) Register(command string, handler CommandHandler) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.handlers[command]; ok {
		return false
	}
	r.handlers[command] = handler
	return true
}

func (r *CommandRunner) Unregister(command string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.handlers, command)
}

func (r *CommandRunner) getHandler(command string) CommandHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.handlers[command]
}

func (r *CommandRunner) Start(ctx context.Context) {
	// TODO: We need to actually start the grpc server here
	// TODO: define num workers
	go r.processLoop(ctx)
}

func (r *CommandRunner) runAdminServer(ctx context.Context) {
	port := 666
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		// log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterAdminServer(grpcServer, NewAdminServer(r.commandQ))
	grpcServer.Serve(lis)
	// grpcServer.GracefulStop()
}

func (r *CommandRunner) processLoop(ctx context.Context) {

	defer func() {
		// Clean up go routines.
		//
		// drain the command q here, closing all response channels?
		// or close  the queue?
	}()

	for {
		select {
		case command := <-r.commandQ:
			handler := r.getHandler(command.command)
			if handler != nil {
				// TODO: perhaps we should also select on the passed in context here?
				err := handler(command.ctx, command.data)
				command.responseChan <- &CommandResponse{err}
			}
			close(command.responseChan)
		case <-ctx.Done():
			// log.Info("processloop shutting down")
			return
		}
	}

}
