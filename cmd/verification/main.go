package main

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	hotsignature "github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	recovery "github.com/onflow/flow-go/consensus/recovery/protocol"
	followereng "github.com/onflow/flow-go/engine/common/follower"
	synceng "github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/engine/verification/assigner"
	"github.com/onflow/flow-go/engine/verification/assigner/blockconsumer"
	"github.com/onflow/flow-go/engine/verification/fetcher"
	"github.com/onflow/flow-go/engine/verification/fetcher/chunkconsumer"
	vereq "github.com/onflow/flow-go/engine/verification/requester"
	"github.com/onflow/flow-go/engine/verification/verifier"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/buffer"
	"github.com/onflow/flow-go/module/chunks"
	finalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/state/protocol"
	badgerState "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/blocktimer"
	storage "github.com/onflow/flow-go/storage/badger"
)

func main() {
	var (
		followerState protocol.MutableState
		err           error
		receiptLimit  uint // size of execution-receipt/result related memory pools.
		chunkAlpha    uint // number of verifiers assigned per chunk.
		chunkLimit    uint // size of chunk-related memory pools.

		requestInterval    time.Duration // time interval that requester engine tries requesting chunk data packs.
		backoffMinInterval time.Duration // minimum time interval a chunk data pack request waits before dispatching.
		backoffMaxInterval time.Duration // maximum time interval a chunk data pack request waits before dispatching.
		backoffMultiplier  float64       // base of exponent in exponential backoff multiplier for backing off requests for chunk data packs.
		requestTargets     uint64        // maximum number of execution nodes a chunk data pack request is dispatched to.

		blockWorkers uint64 // number of blocks processed in parallel.
		chunkWorkers uint64 // number of chunks processed in parallel.

		chunkStatuses        *stdmap.ChunkStatuses     // used in fetcher engine
		chunkRequests        *stdmap.ChunkRequests     // used in requester engine
		processedChunkIndex  *storage.ConsumerProgress // used in chunk consumer
		processedBlockHeight *storage.ConsumerProgress // used in block consumer
		chunkQueue           *storage.ChunksQueue      // used in chunk consumer

		syncCore                *synchronization.Core // used in follower engine
		pendingBlocks           *buffer.PendingBlocks // used in follower engine
		assignerEngine          *assigner.Engine      // the assigner engine
		fetcherEngine           *fetcher.Engine       // the fetcher engine
		requesterEngine         *vereq.Engine         // the requester engine
		verifierEng             *verifier.Engine      // the verifier engine
		chunkConsumer           *chunkconsumer.ChunkConsumer
		blockConsumer           *blockconsumer.BlockConsumer
		finalizationDistributor *pubsub.FinalizationDistributor
		finalizedHeader         *synceng.FinalizedHeaderCache

		followerEng *followereng.Engine        // the follower engine
		collector   module.VerificationMetrics // used to collect metrics of all engines
	)

	nodeBuilder := cmd.FlowNode(flow.RoleVerification.String())
	nodeBuilder.ExtraFlags(func(flags *pflag.FlagSet) {
		flags.UintVar(&receiptLimit, "receipt-limit", 1000, "maximum number of execution receipts in the memory pool")
		flags.UintVar(&chunkLimit, "chunk-limit", 10000, "maximum number of chunk states in the memory pool")
		flags.UintVar(&chunkAlpha, "chunk-alpha", chunks.DefaultChunkAssignmentAlpha, "number of verifiers should be assigned to each chunk")
		flags.DurationVar(&requestInterval, "chunk-request-interval", vereq.DefaultRequestInterval, "time interval chunk data pack request is processed")
		flags.DurationVar(&backoffMinInterval, "backoff-min-interval", vereq.DefaultBackoffMinInterval, "min time interval a chunk data pack request waits before dispatching")
		flags.DurationVar(&backoffMaxInterval, "backoff-max-interval", vereq.DefaultBackoffMaxInterval, "min time interval a chunk data pack request waits before dispatching")
		flags.Float64Var(&backoffMultiplier, "backoff-multiplier", vereq.DefaultBackoffMultiplier, "base of exponent in exponential backoff requesting mechanism")
		flags.Uint64Var(&requestTargets, "request-targets", vereq.DefaultRequestTargets, "maximum number of execution nodes a chunk data pack request is dispatched to")
		flags.Uint64Var(&blockWorkers, "block-workers", blockconsumer.DefaultBlockWorkers, "maximum number of blocks being processed in parallel")
		flags.Uint64Var(&chunkWorkers, "chunk-workers", chunkconsumer.DefaultChunkWorkers, "maximum number of execution nodes a chunk data pack request is dispatched to")

	})

	if err = nodeBuilder.Initialize(); err != nil {
		nodeBuilder.Logger.Fatal().Err(err).Send()
	}

	nodeBuilder.
		Module("mutable follower state", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			// For now, we only support state implementations from package badger.
			// If we ever support different implementations, the following can be replaced by a type-aware factory
			state, ok := node.State.(*badgerState.State)
			if !ok {
				return fmt.Errorf("only implementations of type badger.State are currently supported but read-only state has type %T", node.State)
			}
			followerState, err = badgerState.NewFollowerState(
				state,
				node.Storage.Index,
				node.Storage.Payloads,
				node.Tracer,
				node.ProtocolEvents,
				blocktimer.DefaultBlockTimer,
			)
			return err
		}).
		Module("verification metrics", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			collector = metrics.NewVerificationCollector(node.Tracer, node.MetricsRegisterer)
			return nil
		}).
		Module("chunk status memory pool", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			chunkStatuses = stdmap.NewChunkStatuses(chunkLimit)
			err = node.Metrics.Mempool.Register(metrics.ResourceChunkStatus, chunkStatuses.Size)
			if err != nil {
				return fmt.Errorf("could not register backend metric: %w", err)
			}
			return nil
		}).
		Module("chunk requests memory pool", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			chunkRequests = stdmap.NewChunkRequests(chunkLimit)
			err = node.Metrics.Mempool.Register(metrics.ResourceChunkRequest, chunkRequests.Size)
			if err != nil {
				return fmt.Errorf("could not register backend metric: %w", err)
			}
			return nil
		}).
		Module("processed chunk index consumer progress", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			processedChunkIndex = storage.NewConsumerProgress(node.DB, module.ConsumeProgressVerificationChunkIndex)
			return nil
		}).
		Module("processed block height consumer progress", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			processedBlockHeight = storage.NewConsumerProgress(node.DB, module.ConsumeProgressVerificationBlockHeight)
			return nil
		}).
		Module("chunks queue", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			chunkQueue = storage.NewChunkQueue(node.DB)
			ok, err := chunkQueue.Init(chunkconsumer.DefaultJobIndex)
			if err != nil {
				return fmt.Errorf("could not initialize default index in chunks queue: %w", err)
			}

			node.Logger.Info().
				Str("component", "node-builder").
				Bool("init_to_default", ok).
				Msg("chunks queue index has been initialized")

			return nil
		}).
		Module("pending block cache", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			// consensus cache for follower engine
			pendingBlocks = buffer.NewPendingBlocks()

			// registers size method of backend for metrics
			err = node.Metrics.Mempool.Register(metrics.ResourcePendingBlock, pendingBlocks.Size)
			if err != nil {
				return fmt.Errorf("could not register backend metric: %w", err)
			}

			return nil
		}).
		Module("sync core", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			syncCore, err = synchronization.New(node.Logger, synchronization.DefaultConfig())
			return err
		}).
		Component("verifier engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			rt := fvm.NewInterpreterRuntime()
			vm := fvm.NewVirtualMachine(rt)
			vmCtx := fvm.NewContext(node.Logger, node.FvmOptions...)
			chunkVerifier := chunks.NewChunkVerifier(vm, vmCtx, node.Logger)
			approvalStorage := storage.NewResultApprovals(node.Metrics.Cache, node.DB)
			verifierEng, err = verifier.New(
				node.Logger,
				collector,
				node.Tracer,
				node.Network,
				node.State,
				node.Me,
				chunkVerifier,
				approvalStorage)
			return verifierEng, err
		}).
		Component("chunk consumer, requester, and fetcher engines", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			requesterEngine, err = vereq.New(
				node.Logger,
				node.State,
				node.Network,
				node.Tracer,
				collector,
				chunkRequests,
				requestInterval,
				vereq.RetryAfterQualifier,
				mempool.ExponentialUpdater(backoffMultiplier, backoffMaxInterval, backoffMinInterval),
				requestTargets)

			fetcherEngine = fetcher.New(
				node.Logger,
				collector,
				node.Tracer,
				verifierEng,
				node.State,
				chunkStatuses,
				node.Storage.Headers,
				node.Storage.Blocks,
				node.Storage.Results,
				node.Storage.Receipts,
				requesterEngine)

			// requester and fetcher engines are started by chunk consumer
			chunkConsumer = chunkconsumer.NewChunkConsumer(
				node.Logger,
				collector,
				processedChunkIndex,
				chunkQueue,
				fetcherEngine,
				chunkWorkers)

			err = node.Metrics.Mempool.Register(metrics.ResourceChunkConsumer, chunkConsumer.Size)
			if err != nil {
				return nil, fmt.Errorf("could not register backend metric: %w", err)
			}

			return chunkConsumer, nil
		}).
		Component("assigner engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			var chunkAssigner module.ChunkAssigner
			chunkAssigner, err = chunks.NewChunkAssigner(chunkAlpha, node.State)
			if err != nil {
				return nil, fmt.Errorf("could not initialize chunk assigner: %w", err)
			}

			assignerEngine = assigner.New(
				node.Logger,
				collector,
				node.Tracer,
				node.Me,
				node.State,
				chunkAssigner,
				chunkQueue,
				chunkConsumer)

			return assignerEngine, nil
		}).
		Component("block consumer", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			var initBlockHeight uint64

			blockConsumer, initBlockHeight, err = blockconsumer.NewBlockConsumer(
				node.Logger,
				collector,
				processedBlockHeight,
				node.Storage.Blocks,
				node.State,
				assignerEngine,
				blockWorkers)

			if err != nil {
				return nil, fmt.Errorf("could not initialize block consumer: %w", err)
			}

			err = node.Metrics.Mempool.Register(metrics.ResourceBlockConsumer, blockConsumer.Size)
			if err != nil {
				return nil, fmt.Errorf("could not register backend metric: %w", err)
			}

			node.Logger.Info().
				Str("component", "node-builder").
				Uint64("init_height", initBlockHeight).
				Msg("block consumer initialized")

			return blockConsumer, nil
		}).
		Component("follower engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {

			// initialize cleaner for DB
			cleaner := storage.NewCleaner(node.Logger, node.DB, node.Metrics.CleanCollector, flow.DefaultValueLogGCFrequency)

			// create a finalizer that handles updating the protocol
			// state when the follower detects newly finalized blocks
			final := finalizer.NewFinalizer(node.DB, node.Storage.Headers, followerState, node.Tracer)

			// initialize consensus committee's membership state
			// This committee state is for the HotStuff follower, which follows the MAIN CONSENSUS Committee
			// Note: node.Me.NodeID() is not part of the consensus committee
			committee, err := committees.NewConsensusCommittee(node.State, node.Me.NodeID())
			if err != nil {
				return nil, fmt.Errorf("could not create Committee state for main consensus: %w", err)
			}

			packer := hotsignature.NewConsensusSigDataPacker(committee)
			// initialize the verifier for the protocol consensus
			verifier := verification.NewCombinedVerifier(committee, packer)

			finalized, pending, err := recovery.FindLatest(node.State, node.Storage.Headers)
			if err != nil {
				return nil, fmt.Errorf("could not find latest finalized block and pending blocks to recover consensus follower: %w", err)
			}

			finalizationDistributor = pubsub.NewFinalizationDistributor()
			finalizationDistributor.AddConsumer(blockConsumer)

			// creates a consensus follower with ingestEngine as the notifier
			// so that it gets notified upon each new finalized block
			followerCore, err := consensus.NewFollower(node.Logger, committee, node.Storage.Headers, final, verifier, finalizationDistributor, node.RootBlock.Header,
				node.RootQC, finalized, pending)
			if err != nil {
				return nil, fmt.Errorf("could not create follower core logic: %w", err)
			}

			followerEng, err = followereng.New(
				node.Logger,
				node.Network,
				node.Me,
				node.Metrics.Engine,
				node.Metrics.Mempool,
				cleaner,
				node.Storage.Headers,
				node.Storage.Payloads,
				followerState,
				pendingBlocks,
				followerCore,
				syncCore,
				node.Tracer,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create follower engine: %w", err)
			}

			return followerEng, nil
		}).
		Component("finalized snapshot", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			finalizedHeader, err = synceng.NewFinalizedHeaderCache(node.Logger, node.State, finalizationDistributor)
			if err != nil {
				return nil, fmt.Errorf("could not create finalized snapshot cache: %w", err)
			}

			return finalizedHeader, nil
		}).
		Component("sync engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			sync, err := synceng.New(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.Me,
				node.Storage.Blocks,
				followerEng,
				syncCore,
				finalizedHeader,
				node.SyncEngineIdentifierProvider,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create synchronization engine: %w", err)
			}

			return sync, nil
		}).
		Run()
}
