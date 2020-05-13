package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/onflow/cadence/runtime"
	"github.com/spf13/pflag"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/execution/computation"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/ingestion"
	"github.com/dapperlabs/flow-go/engine/execution/provider"
	"github.com/dapperlabs/flow-go/engine/execution/rpc"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/state/bootstrap"
	"github.com/dapperlabs/flow-go/engine/execution/sync"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/storage/ledger"
)

func main() {

	var (
		ledgerStorage      storage.Ledger
		events             storage.Events
		txResults          storage.TransactionResults
		providerEngine     *provider.Engine
		computationManager *computation.Manager
		ingestionEng       *ingestion.Engine
		rpcConf            rpc.Config
		err                error
		executionState     state.ExecutionState
		triedir            string
		collector          module.ExecutionMetrics
		useMtrie           bool
		mTrieCacheSize     uint32
	)

	cmd.FlowNode("execution").
		ExtraFlags(func(flags *pflag.FlagSet) {
			homedir, _ := os.UserHomeDir()
			datadir := filepath.Join(homedir, ".flow", "execution")

			flags.StringVarP(&rpcConf.ListenAddr, "rpc-addr", "i", "localhost:9000", "the address the gRPC server listens on")
			flags.StringVar(&triedir, "triedir", datadir, "directory to store the execution State")
			flags.BoolVar(&useMtrie, "mtrie", false, "use experimental MTrie for Execution State")
			flags.Uint32Var(&mTrieCacheSize, "mtrie-cache-size", 1000, "cache size for MTrie")
		}).
		Module("computation manager", func(node *cmd.FlowNodeBuilder) error {
			rt := runtime.NewInterpreterRuntime()
			vm, err := virtualmachine.New(rt)
			if err != nil {
				return err
			}

			computationManager = computation.New(
				node.Logger,
				node.Tracer,
				node.Me,
				node.State,
				vm,
			)

			return nil
		}).
		// Trie storage is required to bootstrap, but also should be handled while shutting down
		Module("ledger storage", func(node *cmd.FlowNodeBuilder) error {
			if useMtrie {
				ledgerStorage, err = ledger.NewMTrieStorage(triedir, int(mTrieCacheSize), node.MetricsRegisterer)
			} else {
				ledgerStorage, err = ledger.NewTrieStorage(triedir)
			}

			return err
		}).
		Module("execution metrics", func(node *cmd.FlowNodeBuilder) error {
			collector = metrics.NewExecutionCollector(node.Tracer)
			return nil
		}).
		GenesisHandler(func(node *cmd.FlowNodeBuilder, block *flow.Block) {
			bootstrappedStateCommitment, err := bootstrap.BootstrapLedger(ledgerStorage)
			if err != nil {
				panic(fmt.Sprintf("error while bootstrapping execution state: %s", err))
			}
			if !bytes.Equal(bootstrappedStateCommitment, flow.GenesisStateCommitment) {
				panic("error while bootstrapping execution state - resulting state is different than precalculated!")
			}
			if !bytes.Equal(flow.GenesisStateCommitment, node.GenesisCommit) {
				panic(fmt.Sprintf("genesis seal state commitment (%x) different from precalculated (%x)", node.GenesisCommit, flow.GenesisStateCommitment))
			}

			err = bootstrap.BootstrapExecutionDatabase(node.DB, bootstrappedStateCommitment, block.Header)
			if err != nil {
				panic(fmt.Sprintf("error while boostrapping execution state - cannot bootstrap database: %s", err))
			}
		}).
		Component("execution state ledger", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			return ledgerStorage, nil
		}).
		Component("provider engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			chunkDataPacks := badger.NewChunkDataPacks(node.DB)
			executionResults := badger.NewExecutionResults(node.DB)
			stateCommitments := badger.NewCommits(node.Metrics.Cache, node.DB)

			executionState = state.NewExecutionState(
				ledgerStorage,
				stateCommitments,
				node.Storage.Blocks,
				chunkDataPacks,
				executionResults,
				node.DB,
				node.Tracer,
			)

			stateSync := sync.NewStateSynchronizer(executionState)

			providerEngine, err = provider.New(
				node.Logger,
				node.Tracer,
				node.Network,
				node.State,
				node.Me,
				executionState,
				stateSync,
			)

			return providerEngine, err
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			// Only needed for ingestion engine
			collections := badger.NewCollections(node.DB)

			// Needed for gRPC server, make sure to assign to main scoped vars
			events = badger.NewEvents(node.DB)
			txResults = badger.NewTransactionResults(node.DB)
			ingestionEng, err = ingestion.New(
				node.Logger,
				node.Network,
				node.Me,
				node.State,
				node.Storage.Blocks,
				node.Storage.Payloads,
				collections,
				events,
				txResults,
				computationManager,
				providerEngine,
				executionState,
				6, // TODO - config param maybe?
				collector,
				node.Tracer,
				true,
				time.Second, //TODO - config param
				10,          // TODO - config param
			)
			return ingestionEng, err
		}).
		Component("grpc server", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			rpcEng := rpc.New(node.Logger, rpcConf, ingestionEng, node.Storage.Blocks, events, txResults)
			return rpcEng, nil
		}).Run(flow.RoleExecution.String())

}
