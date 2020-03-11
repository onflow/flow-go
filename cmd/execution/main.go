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
		stateCommitments   storage.Commits
		levelDB          *leveldb.LevelDB
		ledgerStorage      storage.Ledger
		providerEngine     *provider.Engine
		computationManager *computation.Manager
		ingestionEng       *ingestion.Engine
		rpcConf            rpc.Config
		err                error
		executionState     state.ExecutionState
	)

	cmd.FlowNode("execution").
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.StringVarP(&rpcConf.ListenAddr, "rpc-addr", "i", "localhost:9000", "the address the gRPC server listens on")
		}).
		Module("leveldb key-value store", func(node *cmd.FlowNodeBuilder) error {
			levelDB, err = leveldb.NewLevelDB("db/valuedb", "db/triedb")
			return err
		}).
		Module("execution state ledger", func(node *cmd.FlowNodeBuilder) error {
			ledgerStorage, err = ledger.NewTrieStorage(levelDB)
			return err
		}).
		Component("provider engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			chunkHeaders := badger.NewChunkHeaders(node.DB)
			executionResults := badger.NewExecutionResults(node.DB)
			stateCommitments = badger.NewCommits(node.DB)
			executionState = state.NewExecutionState(ledgerStorage, stateCommitments, chunkHeaders, executionResults)
			providerEngine, err = provider.New(
				node.Logger,
				node.Network,
				node.State,
				node.Me,
				executionState,
			)

			return providerEngine, err
		}).
		Component("execution engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			rt := runtime.NewInterpreterRuntime()
			vm := virtualmachine.New(rt)
			executionEng, err = computation.New(
				node.Logger,
				node.Network,
				node.Me,
				node.State,
				receiptsEng,
				vm,
			)

			return executionEng, err
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
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
				computationManager,
				providerEngine,
				executionState,
			)
			return ingestionEng, err
		}).
		Component("grpc server", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			rpcEng := rpc.New(node.Logger, rpcConf, ingestionEng)
			return rpcEng, nil
		}).Run()

}
