package main

import (
	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/execution/execution"
	"github.com/dapperlabs/flow-go/engine/execution/execution/executor"
	"github.com/dapperlabs/flow-go/engine/execution/execution/virtualmachine"
	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/module"
	storage "github.com/dapperlabs/flow-go/storage/mock"
)

func main() {

	cmd.
		FlowNode("execution").
		Component("execution engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {

			node.Logger.Info().Msg("initializing execution engine")

			rt := runtime.NewInterpreterRuntime()
			vm := virtualmachine.New(rt)
			blockExec := executor.NewBlockExecutor(vm)

			// TODO: replace mocks with real implementation
			transactions := &storage.Transactions{}
			collections := &storage.Collections{}

			engine, err := execution.New(
				node.Logger,
				node.Network,
				node.Me,
				collections,
				transactions,
				blockExec,
			)
			node.MustNot(err).Msg("could not initialize execution engine")
			return engine
		}).Run()

}
