package admin

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/onflow/flow-go/admin/admin"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
)

var _ module.Component = (*CommandRunner)(nil)

const (
	CommandRunnerMaxQueueLength  = 128
	CommandRunnerNumWorkers      = 1
	CommandRunnerShutdownTimeout = 5 * time.Second
)

type CommandHandler func(ctx context.Context, data map[string]interface{}) error
type CommandValidator func(data map[string]interface{}) error
type CommandRunnerOption func(*CommandRunner)

func WithTLS(config *tls.Config) CommandRunnerOption {
	return func(r *CommandRunner) {
		r.tlsConfig = config
	}
}

func WithGRPCAddress(address string) CommandRunnerOption {
	return func(r *CommandRunner) {
		r.grpcAddress = address
	}
}

type CommandRunnerBootstrapper struct {
	handlers   map[string]CommandHandler
	validators map[string]CommandValidator
}

func NewCommandRunnerBootstrapper() *CommandRunnerBootstrapper {
	return &CommandRunnerBootstrapper{
		handlers:   make(map[string]CommandHandler),
		validators: make(map[string]CommandValidator),
	}
}

func (r *CommandRunnerBootstrapper) Bootstrap(logger zerolog.Logger, bindAddress string, opts ...CommandRunnerOption) *CommandRunner {
	commandRunner := &CommandRunner{
		handlers:         r.handlers,
		validators:       r.validators,
		grpcAddress:      fmt.Sprintf("%s/flow-node-admin.sock", os.TempDir()),
		httpAddress:      bindAddress,
		logger:           logger.With().Str("admin", "command_runner").Logger(),
		startupCompleted: make(chan struct{}),
	}

	for _, opt := range opts {
		opt(commandRunner)
	}

	return commandRunner
}

func (r *CommandRunnerBootstrapper) RegisterHandler(command string, handler CommandHandler) bool {
	if _, ok := r.handlers[command]; ok {
		return false
	}
	r.handlers[command] = handler
	return true
}

func (r *CommandRunnerBootstrapper) RegisterValidator(command string, validator CommandValidator) bool {
	if _, ok := r.validators[command]; ok {
		return false
	}
	r.validators[command] = validator
	return true
}

type CommandRunner struct {
	handlers    map[string]CommandHandler
	validators  map[string]CommandValidator
	grpcAddress string
	httpAddress string
	tlsConfig   *tls.Config
	logger      zerolog.Logger

	// wait for worker routines to be ready
	workersStarted sync.WaitGroup

	// wait for worker routines to exit
	workersFinished sync.WaitGroup

	// signals startup completion
	startupCompleted chan struct{}
}

func (r *CommandRunner) getHandler(command string) CommandHandler {
	return r.handlers[command]
}

func (r *CommandRunner) getValidator(command string) CommandValidator {
	return r.validators[command]
}

func (r *CommandRunner) Start(ctx irrecoverable.SignalerContext) error {
	if err := r.runAdminServer(ctx); err != nil {
		return fmt.Errorf("failed to start admin server: %w", err)
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

func (r *CommandRunner) runAdminServer(ctx irrecoverable.SignalerContext) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	r.logger.Info().Msg("admin server starting up")

	listener, err := net.Listen("unix", r.grpcAddress)
	if err != nil {
		return fmt.Errorf("failed to listen on admin server address: %w", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterAdminServer(grpcServer, NewAdminServer(r))

	r.workersStarted.Add(1)
	r.workersFinished.Add(1)
	go func() {
		defer r.workersFinished.Done()
		r.workersStarted.Done()

		if err := grpcServer.Serve(listener); err != nil {
			r.logger.Err(err).Msg("gRPC server encountered fatal error")
			ctx.Throw(err)
		}
	}()

	// Register gRPC server endpoint
	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithInsecure()}

	err = pb.RegisterAdminHandlerFromEndpoint(ctx, mux, "unix:///"+r.grpcAddress, opts)
	if err != nil {
		return fmt.Errorf("failed to register http handlers for admin service: %w", err)
	}

	httpServer := &http.Server{
		Addr:      r.httpAddress,
		Handler:   mux,
		TLSConfig: r.tlsConfig,
	}

	r.workersStarted.Add(1)
	r.workersFinished.Add(1)
	go func() {
		defer r.workersFinished.Done()
		r.workersStarted.Done()

		// Start HTTP server (and proxy calls to gRPC server endpoint)
		var err error
		if r.tlsConfig == nil {
			err = httpServer.ListenAndServe()
		} else {
			err = httpServer.ListenAndServeTLS("", "")
		}

		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			r.logger.Err(err).Msg("HTTP server encountered error")
			ctx.Throw(err)
		}
	}()

	r.workersStarted.Add(1)
	r.workersFinished.Add(1)
	go func() {
		defer r.workersFinished.Done()
		r.workersStarted.Done()

		<-ctx.Done()
		r.logger.Info().Msg("admin server shutting down")

		grpcServer.Stop()

		if httpServer != nil {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), CommandRunnerShutdownTimeout)
			defer shutdownCancel()

			if err := httpServer.Shutdown(shutdownCtx); err != nil {
				r.logger.Err(err).Msg("failed to shutdown http server")
				ctx.Throw(err)
			}
		}
	}()

	return nil
}

func (r *CommandRunner) runCommand(ctx context.Context, command string, data map[string]interface{}) error {
	r.logger.Info().Str("command", command).Msg("received new command")

	if validator := r.getValidator(command); validator != nil {
		if validationErr := validator(data); validationErr != nil {
			return status.Error(codes.InvalidArgument, validationErr.Error())
		}
	}

	if handler := r.getHandler(command); handler != nil {
		if handleErr := handler(ctx, data); handleErr != nil {
			if errors.Is(handleErr, context.Canceled) {
				return status.Error(codes.Canceled, "client canceled")
			} else if errors.Is(handleErr, context.DeadlineExceeded) {
				return status.Error(codes.DeadlineExceeded, "request timed out")
			} else {
				s, _ := status.FromError(handleErr)
				return s.Err()
			}
		}
	} else {
		return status.Error(codes.Unimplemented, "invalid command")
	}

	return nil
}
