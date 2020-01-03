package main

import (
	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/execution/execution"
	"github.com/dapperlabs/flow-go/engine/execution/execution/executor"
	"github.com/dapperlabs/flow-go/engine/execution/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/execution/virtualmachine"
	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/storage/ledger"
	storage "github.com/dapperlabs/flow-go/storage/mock"
)

func main() {

	cmd.
		FlowNode("execution").
		Component("execution engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {

			node.Logger.Info().Msg("initializing execution engine")

			rt := runtime.NewInterpreterRuntime()
			vm := virtualmachine.New(rt)
			ls, err := ledger.NewTrieStorage()
			node.MustNot(err).Msg("could not initialize ledger trie storage")

			execState := state.NewExecutionState(ls)

			blockExec := executor.NewBlockExecutor(vm, execState)

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
