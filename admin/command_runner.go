package admin

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/onflow/flow-go/admin/admin"
	"github.com/rs/zerolog"
)

const (
	CommandRunnerMaxQueueLength = 128
	CommandRunnerNumWorkers     = 1
)

type CommandRunner struct {
	handlers   map[string]CommandHandler
	validators map[string]CommandValidator
	commandQ   chan *CommandRequest
	address    string
	logger     zerolog.Logger

	// mutex to guard against concurrent access to handlers and validators
	mu sync.RWMutex

	// wait for worker routines to be ready
	workersStarted sync.WaitGroup

	// wait for worker routines to exit
	workersFinished sync.WaitGroup

	// signals startup completion
	startupCompleted chan struct{}
}

type CommandHandler func(ctx context.Context, data map[string]interface{}) error
type CommandValidator func(data map[string]interface{}) error

func NewCommandRunner(logger zerolog.Logger, address string) *CommandRunner {
	return &CommandRunner{
		handlers:         make(map[string]CommandHandler),
		validators:       make(map[string]CommandValidator),
		commandQ:         make(chan *CommandRequest, CommandRunnerMaxQueueLength),
		address:          address,
		logger:           logger.With().Str("admin", "command_runner").Logger(),
		startupCompleted: make(chan struct{}),
	}
}

func (r *CommandRunner) RegisterHandler(command string, handler CommandHandler) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.handlers[command]; ok {
		return false
	}
	r.handlers[command] = handler
	return true
}

func (r *CommandRunner) UnregisterHandler(command string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.handlers, command)
}

func (r *CommandRunner) RegisterValidator(command string, validator CommandValidator) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.validators[command]; ok {
		return false
	}
	r.validators[command] = validator
	return true
}

func (r *CommandRunner) UnregisterValidator(command string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.validators, command)
}

func (r *CommandRunner) getHandler(command string) CommandHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.handlers[command]
}

func (r *CommandRunner) getValidator(command string) CommandValidator {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.validators[command]
}

func (r *CommandRunner) Start(ctx context.Context) error {
	if err := r.runAdminServer(ctx); err != nil {
		return fmt.Errorf("failed to start admin server: %w", err)
	}

	for i := 0; i < CommandRunnerNumWorkers; i++ {
		r.workersStarted.Add(1)
		r.workersFinished.Add(1)
		go r.processLoop(ctx)
	}

	close(r.startupCompleted)

	return nil
}

func (r *CommandRunner) Ready() <-chan struct{} {
	ready := make(chan struct{})

	go func() {
		<-r.startupCompleted
		r.workersStarted.Wait()
		close(ready)
	}()

	return ready
}

func (r *CommandRunner) Done() <-chan struct{} {
	done := make(chan struct{})

	go func() {
		<-r.startupCompleted
		r.workersFinished.Wait()
		close(done)
	}()

	return done
}

func (r *CommandRunner) runAdminServer(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	r.logger.Info().Msg("admin server starting up")

	listener, err := net.Listen("tcp", r.address)
	if err != nil {
		return fmt.Errorf("failed to listen on admin server address: %w", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterAdminServer(grpcServer, NewAdminServer(r.commandQ))

	r.workersStarted.Add(2)
	r.workersFinished.Add(2)
	go func() {
		defer r.workersFinished.Done()
		r.workersStarted.Done()

		if err := grpcServer.Serve(listener); err != nil {
			r.logger.Err(err).Msg("gRPC server encountered fatal error")
		}
	}()
	go func() {
		defer r.workersFinished.Done()
		r.workersStarted.Done()

		<-ctx.Done()
		r.logger.Info().Msg("admin server shutting down")

		grpcServer.Stop()
		close(r.commandQ)
	}()

	return nil
}

func (r *CommandRunner) processLoop(ctx context.Context) {
	defer func() {
		// cleanup uncompleted requests from the command queue
		for command := range r.commandQ {
			close(command.responseChan)
		}

		r.workersFinished.Done()
	}()

	r.workersStarted.Done()

	for {
		select {
		case command := <-r.commandQ:
			r.logger.Info().Str("command", command.command).Msg("received new command")

			var err error

			if validator := r.getValidator(command.command); validator != nil {
				if validationErr := validator(command.data); validationErr != nil {
					err = status.Error(codes.InvalidArgument, validationErr.Error())
					goto sendResponse
				}
			}

			if handler := r.getHandler(command.command); handler != nil {
				// TODO: we can probably merge the command context with the worker context
				// using something like: https://github.com/teivah/onecontext
				if handleErr := handler(command.ctx, command.data); handleErr != nil {
					if errors.Is(handleErr, context.Canceled) {
						err = status.Error(codes.Canceled, "client canceled")
					} else if errors.Is(handleErr, context.DeadlineExceeded) {
						err = status.Error(codes.DeadlineExceeded, "request timed out")
					} else {
						s, _ := status.FromError(handleErr)
						err = s.Err()
					}
				}
			} else {
				err = status.Error(codes.Unimplemented, "invalid command")
			}

		sendResponse:
			command.responseChan <- &CommandResponse{err}
			close(command.responseChan)
		case <-ctx.Done():
			r.logger.Info().Msg("process loop shutting down")
			return
		}
	}

}
