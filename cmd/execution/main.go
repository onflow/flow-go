package main

import (
	"github.com/spf13/pflag"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/execution/execution"
	"github.com/dapperlabs/flow-go/engine/execution/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/execution/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/ingestion"
	"github.com/dapperlabs/flow-go/engine/execution/provider"
	"github.com/dapperlabs/flow-go/engine/execution/rpc"
	"github.com/dapperlabs/flow-go/engine/execution/sync"
	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/storage/ledger/databases/leveldb"
)

func main() {

	var (
		stateCommitments storage.Commits
		ledgerStorage    storage.Ledger
		receiptsEng      *provider.Engine
		executionEng     *execution.Engine
		ingestionEng     *ingestion.Engine
		rpcConf          rpc.Config
		err              error
		executionState   state.ExecutionState
	)

	cmd.
		FlowNode("execution").
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.StringVarP(&rpcConf.ListenAddr, "rpc-addr", "i", "localhost:9000", "the address the gRPC server listens on")
		}).
		PostInit(func(node *cmd.FlowNodeBuilder) {
			stateCommitments = badger.NewCommits(node.DB)

			levelDB, err := leveldb.NewLevelDB("db/valuedb", "db/triedb")
			node.MustNot(err).Msg("could not initialize LevelDB databases")

			ledgerStorage, err = ledger.NewTrieStorage(levelDB)
			node.MustNot(err).Msg("could not initialize ledger trie storage")
		}).
		Component("receipts engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing receipts engine")

			chunkHeaders := badger.NewChunkHeaders(node.DB)
			registerDeltas := badger.NewRegisterDeltas(node.DB)

			executionState = state.NewExecutionState(ledgerStorage, stateCommitments, chunkHeaders)

			stateSync := sync.NewStateSynchronizer(node.State, registerDeltas)

			receiptsEng, err = provider.New(
				node.Logger,
				node.Network,
				node.State,
				node.Me,
				executionState,
				stateSync,
			)
			node.MustNot(err).Msg("could not initialize receipts engine")

			return receiptsEng
		}).
		Component("execution engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing execution engine")

			rt := runtime.NewInterpreterRuntime()
			vm := virtualmachine.New(rt)

			executionEng, err = execution.New(
				node.Logger,
				node.Network,
				node.Me,
				node.State,
				receiptsEng,
				vm,
			)
			node.MustNot(err).Msg("could not initialize execution engine")

			return executionEng
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing ingestion engine")

			blocks := badger.NewBlocks(node.DB)
			collections := badger.NewCollections(node.DB)

			ingestionEng, err = ingestion.New(
				node.Logger,
				node.Network,
				node.Me,
				node.State,
				blocks,
				collections,
				executionEng,
				executionState,
			)
			node.MustNot(err).Msg("could not initialize ingestion engine")

			return ingestionEng
		}).
		Component("RPC engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing gRPC server")

			rpcEng := rpc.New(node.Logger, rpcConf, ingestionEng)
			return rpcEng
		}).Run()

}
