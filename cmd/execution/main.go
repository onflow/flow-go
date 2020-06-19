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
	"github.com/dapperlabs/flow-go/storage/ledger/wal"
)

func main() {

	var (
		ledgerStorage      *ledger.MTrieStorage
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
		mTrieCacheSize     uint32
	)

	cmd.FlowNode(flow.RoleExecution.String()).
		ExtraFlags(func(flags *pflag.FlagSet) {
			homedir, _ := os.UserHomeDir()
			datadir := filepath.Join(homedir, ".flow", "execution")

			flags.StringVarP(&rpcConf.ListenAddr, "rpc-addr", "i", "localhost:9000", "the address the gRPC server listens on")
			flags.StringVar(&triedir, "triedir", datadir, "directory to store the execution State")
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
				node.Storage.Blocks,
			)

			return nil
		}).
		Module("execution metrics", func(node *cmd.FlowNodeBuilder) error {
			collector = metrics.NewExecutionCollector(node.Tracer, node.MetricsRegisterer)
			return nil
		}).
		// Trie storage is required to bootstrap, but also should be handled while shutting down
		Module("ledger storage", func(node *cmd.FlowNodeBuilder) error {
			ledgerStorage, err = ledger.NewMTrieStorage(triedir, int(mTrieCacheSize), collector, node.MetricsRegisterer)
			return err
		}).
		GenesisHandler(func(node *cmd.FlowNodeBuilder, block *flow.Block) {
			if node.GenesisAccountPublicKey == nil {
				panic(fmt.Sprintf("error while bootstrapping execution state: no service account public key"))
			}

			bootstrappedStateCommitment, err := bootstrap.BootstrapLedger(ledgerStorage, *node.GenesisAccountPublicKey, node.GenesisTokenSupply)
			if err != nil {
				panic(fmt.Sprintf("error while bootstrapping execution state: %s", err))
			}

			if !bytes.Equal(bootstrappedStateCommitment, node.GenesisCommit) {
				panic(fmt.Sprintf("genesis seal state commitment (%x) different from precalculated (%x)", bootstrappedStateCommitment, node.GenesisCommit))
			}

			err = bootstrap.BootstrapExecutionDatabase(node.DB, bootstrappedStateCommitment, block.Header)
			if err != nil {
				panic(fmt.Sprintf("error while bootstrapping execution state - cannot bootstrap database: %s", err))
			}
		}).
		Component("execution state ledger", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			return ledgerStorage, nil
		}).
		Component("execution state ledger WAL compactor", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			checkpointer, err := ledgerStorage.Checkpointer()
			if err != nil {
				return nil, fmt.Errorf("cannot create checkpointer: %w", err)
			}
			compactor := wal.NewCompactor(checkpointer, 10*time.Second)

			return compactor, nil
		}).
		Component("provider engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			chunkDataPacks := badger.NewChunkDataPacks(node.DB)
			executionResults := badger.NewExecutionResults(node.DB)
			stateCommitments := badger.NewCommits(node.Metrics.Cache, node.DB)

			executionState = state.NewExecutionState(
				ledgerStorage,
				stateCommitments,
				node.Storage.Blocks,
				node.Storage.Collections,
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
				node.Storage.Collections,
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
		}).Run()

}
