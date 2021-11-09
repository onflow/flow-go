package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/dgraph-io/badger/v2"
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

// FlowNodeImp is created by the FlowNodeBuilder with all components ready to be
// started.
// The Run function starts all the components, and is blocked until
// either a termination signal is received or a irrecoverable error is countered.
// If either case happened, it stops the database and exit.
type FlowNodeImp struct {
	*component.ComponentManager
	*NodeConfig
	Logger zerolog.Logger
	DB     *badger.DB
}

// Run calls Start() to start all the node components. It also sets up a channel to gracefully shut
// down each component if a SIGTERM is received. Until a SIGTERM is received, Run will block.
// Since, Run is a blocking call it should only be used when running a node as it's own independent process.
// Any unhandled irrecoverable errors thrown in child components will propagate up to here and result in a fatal
// error
func (node *FlowNodeImp) Run() {
	// blocking
	err := node.run()

	if err != nil {
		dbErr := node.closeDatabase()
		if dbErr != nil {
			node.Logger.Fatal().Err(err).Msgf("failed to close database: %v", dbErr)
		}

		node.Logger.Fatal().Err(err).Msgf("exit now")
	}

	node.Logger.Info().Msgf("database closed, %s node has stopped gracefully", node.BaseConfig.NodeRole)
	os.Exit(0)
}

// It returns:
//   - nil if a termination signal is received, and all components have been gracefully stopped.
//   - error if a irrecoverable error is received, or failed to stop a component gracefully
func (node *FlowNodeImp) run() error {
	// startComponents starts the components and returns a channel of irrecoverable error
	// and a function to stop all components
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)
	go node.Start(signalerCtx)

	// log when all components have been started
	go func() {
		select {
		case <-node.Ready():
			node.Logger.Info().Msgf("%s node startup complete", node.BaseConfig.NodeRole)
		case <-ctx.Done():
		}
	}()

	// listen to termination signal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	sigCtx, _ := util.WithSignal(ctx, signalChan)

	// block till termination signal is received or a fatal error is encountered
	err := util.WaitError(sigCtx, errChan)

	// if fatal error is encountered, return the fatal error
	if err != nil {
		cancel() // prevent context leak
		return fmt.Errorf("unhandled irrecoverable error: %w", err)
	}

	// if termination signal is received, notify all components to stop gracefully
	node.Logger.Info().Msgf("received termination signal, graceful shutting down %s node", node.BaseConfig.NodeRole)
	cancel()

	doneCtx, _ := util.WithDone(sigCtx, node.Done())
	// block till either all components have been gracefully stopped or another termination signal is received
	// to force exit
	err = util.WaitError(doneCtx, errChan)

	// if irrecoverable error is encountered during graceful shutdown, return the irrecoverable error
	if err != nil {
		return fmt.Errorf("unhandled irrecoverable error during graceful shutdown: %w", err)
	}

	// if another termination signal is received, start forcing an exit
	if errors.Is(sigCtx.Err(), util.ErrSignalReceived) {
		return fmt.Errorf("received termination signal, aborting graceful shutdown, shutting down now")
	}

	node.Logger.Info().Msgf("%s node's all components have stopped gracefully", node.BaseConfig.NodeRole)
	return nil
}

// Since DB and SecretsDB are the same badger DB instance,
// we only need to stop one of them
func (node *FlowNodeImp) closeDatabase() error {
	return node.DB.Close()
}
