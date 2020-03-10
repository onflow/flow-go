package main

import (
	"github.com/spf13/pflag"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/execution/computation"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/ingestion"
	"github.com/dapperlabs/flow-go/engine/execution/provider"
	"github.com/dapperlabs/flow-go/engine/execution/rpc"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/storage/ledger/databases/leveldb"
)

func main() {

	var (
		stateCommitments  storage.Commits
		ledgerStorage     storage.Ledger
		providerEngine    *provider.Engine
		computationEngine *computation.Engine
		ingestionEng      *ingestion.Engine
		rpcConf           rpc.Config
		err               error
		executionState    state.ExecutionState
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

			rt := runtime.NewInterpreterRuntime()
			vm := virtualmachine.New(rt)
			computationEngine = computation.New(
				node.Logger,
				node.Me,
				node.State,
				vm,
			)
		}).
		Component("receipts engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing receipts engine")

			chunkHeaders := badger.NewChunkHeaders(node.DB)

			chunkDataPacks := badger.NewChunkDataPacks(node.DB)

			executionResults := badger.NewExecutionResults(node.DB)

			executionState = state.NewExecutionState(ledgerStorage, stateCommitments, chunkHeaders, chunkDataPacks, executionResults)

			providerEngine, err = provider.New(
				node.Logger,
				node.Network,
				node.State,
				node.Me,
				executionState,
			)
			node.MustNot(err).Msg("could not initialize receipts engine")

			return providerEngine
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing ingestion engine")

			blocks := badger.NewBlocks(node.DB)
			collections := badger.NewCollections(node.DB)
			payloads := badger.NewPayloads(node.DB)

			ingestionEng, err = ingestion.New(
				node.Logger,
				node.Network,
				node.Me,
				node.State,
				blocks,
				payloads,
				collections,
				computationEngine,
				providerEngine,
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
