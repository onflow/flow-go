package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/onflow/cadence/runtime"
	"github.com/spf13/pflag"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/common/requester"
	"github.com/dapperlabs/flow-go/engine/common/synchronization"
	"github.com/dapperlabs/flow-go/engine/execution/computation"
	"github.com/dapperlabs/flow-go/engine/execution/ingestion"
	"github.com/dapperlabs/flow-go/engine/execution/provider"
	"github.com/dapperlabs/flow-go/engine/execution/rpc"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/state/bootstrap"
	"github.com/dapperlabs/flow-go/engine/execution/sync"
	"github.com/dapperlabs/flow-go/fvm"
	bootstrapFilenames "github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/metrics"
	chainsync "github.com/dapperlabs/flow-go/module/synchronization"
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
		syncCore           *chainsync.Core
		computationManager *computation.Manager
		syncEngine         *synchronization.Engine
		requestEng         *requester.Engine
		ingestionEng       *ingestion.Engine
		rpcConf            rpc.Config
		err                error
		executionState     state.ExecutionState
		triedir            string
		collector          module.ExecutionMetrics
		mTrieCacheSize     uint32
		checkpointDistance uint
	)

	cmd.FlowNode(flow.RoleExecution.String()).
		ExtraFlags(func(flags *pflag.FlagSet) {
			homedir, _ := os.UserHomeDir()
			datadir := filepath.Join(homedir, ".flow", "execution")

			flags.StringVarP(&rpcConf.ListenAddr, "rpc-addr", "i", "localhost:9000", "the address the gRPC server listens on")
			flags.StringVar(&triedir, "triedir", datadir, "directory to store the execution State")
			flags.Uint32Var(&mTrieCacheSize, "mtrie-cache-size", 1000, "cache size for MTrie")
			flags.UintVar(&checkpointDistance, "checkpoint-distance", 1, "number of WAL segments between checkpoints")
		}).
		Module("computation manager", func(node *cmd.FlowNodeBuilder) error {
			rt := runtime.NewInterpreterRuntime()
			vm, err := fvm.New(rt, node.RootChainID.Chain())
			if err != nil {
				return err
			}

			computationManager = computation.New(
				node.Logger,
				collector,
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
		Module("sync core", func(node *cmd.FlowNodeBuilder) error {
			syncCore, err = chainsync.New(node.Logger, chainsync.DefaultConfig())
			return err
		}).
		Component("execution state ledger", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			bootstrapper := bootstrap.NewBootstrapper(node.Logger)
			err := bootstrapper.BootstrapExecutionDatabase(node.DB, node.RootSeal.FinalState, node.RootBlock.Header)
			// Root block already loaded, can simply continued
			if err != nil && !errors.Is(err, storage.ErrAlreadyExists) {
				return nil, fmt.Errorf("could not bootstrap execution DB: %w", err)
			} else if err == nil {
				// Newly bootstrapped Execution DB. Make sure to load execution state
				if err := loadBootstrapState(node.BaseConfig.BootstrapDir, triedir); err != nil {
					return nil, fmt.Errorf("could not load bootstrap state: %w", err)
				}
			}
			ledgerStorage, err = ledger.NewMTrieStorage(triedir, int(mTrieCacheSize), collector, node.MetricsRegisterer)
			return ledgerStorage, err
		}).
		Component("execution state ledger WAL compactor", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			checkpointer, err := ledgerStorage.Checkpointer()
			if err != nil {
				return nil, fmt.Errorf("cannot create checkpointer: %w", err)
			}
			compactor := wal.NewCompactor(checkpointer, 10*time.Second, checkpointDistance)

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

			requestEng, err = requester.New(node.Logger, node.Metrics.Engine, node.Network, node.Me, node.State,
				engine.RequestCollections,
				filter.HasRole(flow.RoleCollection),
			)

			// Needed for gRPC server, make sure to assign to main scoped vars
			events = badger.NewEvents(node.DB)
			txResults = badger.NewTransactionResults(node.DB)
			ingestionEng, err = ingestion.New(
				node.Logger,
				node.Network,
				node.Me,
				requestEng,
				node.State,
				node.Storage.Blocks,
				node.Storage.Payloads,
				node.Storage.Collections,
				events,
				txResults,
				computationManager,
				providerEngine,
				syncCore,
				executionState,
				6, // TODO - config param maybe?
				collector,
				node.Tracer,
				true,
			)

			// TODO: we should solve these mutual dependencies better
			// => https://github.com/dapperlabs/flow-go/issues/4360
			requestEng = requestEng.WithHandle(ingestionEng.OnCollection)

			return ingestionEng, err
		}).
		Component("sychronization engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			// initialize the synchronization engine
			syncEngine, err = synchronization.New(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.Me,
				node.State,
				node.Storage.Blocks,
				ingestionEng,
				syncCore,
			)
			if err != nil {
				return nil, fmt.Errorf("could not initialize synchronization engine: %w", err)
			}

			return syncEngine, nil
		}).
		Component("grpc server", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			rpcEng := rpc.New(node.Logger, rpcConf, ingestionEng, node.Storage.Blocks, events, txResults, node.RootChainID)
			return rpcEng, nil
		}).Run()
}

func loadBootstrapState(dir, trie string) error {
	filename := ""

	if _, err := os.Stat(filepath.Join(dir, bootstrapFilenames.DirnameExecutionState, "checkpoint.00000000")); err == nil {
		filename = "checkpoint.00000000"
	} else if _, err := os.Stat(filepath.Join(dir, bootstrapFilenames.DirnameExecutionState, "00000000")); err == nil {
		filename = "00000000"
	} else {
		return fmt.Errorf("could not find bootstrapped execution state")
	}

	src := filepath.Join(dir, bootstrapFilenames.DirnameExecutionState, filename)
	dst := filepath.Join(trie, filename)

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}
