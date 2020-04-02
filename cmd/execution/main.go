package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dapperlabs/cadence/runtime"

	"github.com/spf13/pflag"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/execution/computation"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/ingestion"
	"github.com/dapperlabs/flow-go/engine/execution/provider"
	"github.com/dapperlabs/flow-go/engine/execution/rpc"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/state/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/storage/ledger/databases/leveldb"
)

func main() {

	var (
		stateCommitments   storage.Commits
		levelDB            *leveldb.LevelDB
		ledgerStorage      storage.Ledger
		providerEngine     *provider.Engine
		computationManager *computation.Manager
		ingestionEng       *ingestion.Engine
		rpcConf            rpc.Config
		err                error
		executionState     state.ExecutionState
		triedir            string
	)

	cmd.FlowNode("execution").
		ExtraFlags(func(flags *pflag.FlagSet) {
			homedir, _ := os.UserHomeDir()
			datadir := filepath.Join(homedir, ".flow", "execution")

			flags.StringVarP(&rpcConf.ListenAddr, "rpc-addr", "i", "localhost:9000", "the address the gRPC server listens on")
			flags.StringVar(&triedir, "triedir", datadir, "directory to store the execution State")
		}).
		Module("leveldb key-value store", func(node *cmd.FlowNodeBuilder) error {
			levelDB, err = leveldb.NewLevelDB(filepath.Join(triedir, "valuedb"), filepath.Join(triedir, "triedb"))
			return err
		}).
		Module("execution state ledger", func(node *cmd.FlowNodeBuilder) error {
			ledgerStorage, err = ledger.NewTrieStorage(levelDB)
			return err
		}).
		Module("computation manager", func(node *cmd.FlowNodeBuilder) error {
			rt := runtime.NewInterpreterRuntime()
			vm := virtualmachine.New(rt)
			computationManager = computation.New(
				node.Logger,
				node.Me,
				node.State,
				vm,
			)

			return nil
		}).
		GenesisHandler(func(node *cmd.FlowNodeBuilder, block *flow.Block) {
			bootstrappedStateCommitment, err := bootstrap.BootstrapLedger(ledgerStorage)
			if err != nil {
				panic(fmt.Sprintf("error while bootstrapping execution state: %s", err))
			}
			if !bytes.Equal(bootstrappedStateCommitment, flow.GenesisStateCommitment) {
				panic("error while boostrapping execution state - resulting state is different than precalculated!")
			}
			if !bytes.Equal(flow.GenesisStateCommitment, block.Seals[0].FinalState) {
				panic("genesis seal state commitment different from precalculated")
			}
		}).
		Component("provider engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			chunkHeaders := badger.NewChunkHeaders(node.DB)
			chunkDataPacks := badger.NewChunkDataPacks(node.DB)
			executionResults := badger.NewExecutionResults(node.DB)
			stateCommitments = badger.NewCommits(node.DB)
			executionState = state.NewExecutionState(ledgerStorage, stateCommitments, chunkHeaders, chunkDataPacks, executionResults)
			providerEngine, err = provider.New(
				node.Logger,
				node.Network,
				node.State,
				node.Me,
				executionState,
			)

			return providerEngine, err
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			blocks := badger.NewBlocks(node.DB)
			collections := badger.NewCollections(node.DB)
			payloads := badger.NewPayloads(node.DB)
			events := badger.NewEvents(node.DB)
			ingestionEng, err = ingestion.New(
				node.Logger,
				node.Network,
				node.Me,
				node.State,
				blocks,
				payloads,
				collections,
				events,
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
