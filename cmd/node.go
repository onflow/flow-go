package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
)

var _ component.Component = (*FlowNodeImp)(nil)

type Node interface {
	component.Component

	// Run initiates all common components (logger, database, protocol state etc.)
	// then starts each component. It also sets up a channel to gracefully shut
	// down each component if a SIGINT is received.
	Run()
}

// FlowNodeImp is created by the FlowNodeBuilder with all components ready to be started.
// The Run function starts all the components, and is blocked until either a termination
// signal is received or a irrecoverable error is encountered.
type FlowNodeImp struct {
	component.Component
	*NodeConfig
	logger       zerolog.Logger
	postShutdown func() error
	fatalHandler func(error, zerolog.Logger)
}

// NewNode returns a new node instance
func NewNode(manager *component.ComponentManager, cfg *NodeConfig, logger zerolog.Logger, f func() error) Node {
	return &FlowNodeImp{
		Component:    manager,
		NodeConfig:   cfg,
		logger:       logger,
		postShutdown: f,
		fatalHandler: handleFatal,
	}
}

// Run starts all the node's components, then blocks until a SIGINT or SIGTERM is received, at
// which point it gracefully shuts down.
// Any unhandled irrecoverable errors thrown in child components will propagate up to here and
// result in a fatal error.
func (node *FlowNodeImp) Run() {
	// Block until node is shutting down
	err := node.run()

	// Any error received is considered fatal.
	if err != nil {
		node.fatalHandler(err, node.logger)
		return
	}

	// Run post shutdown cleanup logic
	err = node.postShutdown()

	// Since this occurs after all components have stopped, it is not considered fatal
	if err != nil {
		node.logger.Error().Err(err).Msg("error encountered during cleanup")
	}

	node.logger.Info().Msgf("%s node shutdown complete", node.BaseConfig.NodeRole)
}

// run starts the node and blocks until a SIGINT/SIGTERM is received or an error is encountered.
// It returns:
//   - nil if a termination signal is received, and all components have been gracefully stopped.
//   - error if a irrecoverable error is received
func (node *FlowNodeImp) run() error {
	ctx, cancel := context.WithCancel(context.Background())

	// 1: Start up
	// Components will pass unhandled irrecoverable errors to this channel via signalerCtx (or a
	// child context). Any errors received on this channel should halt the node.
	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)

	// Start all the components
	node.Start(signalerCtx)

	// Log when all components have been started
	go func() {
		select {
		case <-node.Ready():
			node.logger.Info().Msgf("%s node startup complete", node.BaseConfig.NodeRole)
		case <-ctx.Done():
		}
	}()

	// 2: Run the node
	// This context will be marked done when SIGINT/SIGTERM is received.
	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)

	// Block here until either a signal or irrecoverable error is received.
	err := util.WaitError(sigCtx, errChan)

	// Stop relaying signals. Subsequent signals will be handled by the OS and will abort the
	// process.
	stop()

	// If an irrecoverable error was received, abort
	if err != nil {
		return err
	}

	// Send shutdown signal to components
	node.logger.Info().Msgf("%s node shutting down", node.BaseConfig.NodeRole)
	cancel()

	// 3: Shut down
	// This context will be marked done when all components have stopped
	doneCtx, _ := util.WithDone(context.Background(), node.Done())

	// Block here until all components have stopped or an irrecoverable error is received.
	err = util.WaitError(doneCtx, errChan)
	if err != nil {
		return err
	}

	return nil
}

// handleFatal handles irrecoverable errors by logging them and exiting the process.
// This allows for easier testing and is not intended to be overriden in production.
func handleFatal(err error, logger zerolog.Logger) {
	logger.Fatal().Err(err).Msg("unhandled irrecoverable error")
}
