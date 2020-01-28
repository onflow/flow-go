package main

import (
	"github.com/spf13/pflag"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/execution/execution"
	"github.com/dapperlabs/flow-go/engine/execution/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/execution/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/ingestion"
	"github.com/dapperlabs/flow-go/engine/execution/receipts"
	"github.com/dapperlabs/flow-go/engine/execution/rpc"
	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/model/flow"
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
		receiptsEng      *receipts.Engine
		executionEng     *execution.Engine
		rpcConf          rpc.Config
		err              error
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
		GenesisHandler(func(node *cmd.FlowNodeBuilder, genesis *flow.Block) {
			// TODO We boldly assume that if a genesis is being written than a storage tree is also empty
			initialStateCommitment := ledgerStorage.LatestStateCommitment()

			err := stateCommitments.Store(genesis.ID(), initialStateCommitment)
			node.MustNot(err).Msg("could not store initial state commitment for genesis block")

		}).
		Component("receipts engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing receipts engine")

			receiptsEng, err = receipts.New(
				node.Logger,
				node.Network,
				node.State,
				node.Me,
			)
			node.MustNot(err).Msg("could not initialize receipts engine")

			return receiptsEng
		}).
		Component("execution engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing execution engine")

			rt := runtime.NewInterpreterRuntime()
			vm := virtualmachine.New(rt)

			chunkHeaders := badger.NewChunkHeaders(node.DB)

			execState := state.NewExecutionState(ledgerStorage, stateCommitments, chunkHeaders)

			executionEng, err = execution.New(
				node.Logger,
				node.Network,
				node.Me,
				node.State,
				execState,
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

			ingestionEng, err := ingestion.New(
				node.Logger,
				node.Network,
				node.Me,
				node.State,
				blocks,
				collections,
				executionEng,
			)
			node.MustNot(err).Msg("could not initialize ingestion engine")

			return ingestionEng
		}).
		Component("RPC engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing gRPC server")

			rpcEng := rpc.New(node.Logger, rpcConf, executionEng)
			return rpcEng
		}).Run()

}
