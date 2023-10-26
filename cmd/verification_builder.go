package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"

	flowconsensus "github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	hotsignature "github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/consensus/hotstuff/validator"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	recoveryprotocol "github.com/onflow/flow-go/consensus/recovery/protocol"
	"github.com/onflow/flow-go/engine/common/follower"
	followereng "github.com/onflow/flow-go/engine/common/follower"
	commonsync "github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/engine/verification/assigner"
	"github.com/onflow/flow-go/engine/verification/assigner/blockconsumer"
	"github.com/onflow/flow-go/engine/verification/fetcher"
	"github.com/onflow/flow-go/engine/verification/fetcher/chunkconsumer"
	"github.com/onflow/flow-go/engine/verification/requester"
	"github.com/onflow/flow-go/engine/verification/verifier"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/chainsync"
	"github.com/onflow/flow-go/module/chunks"
	finalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/protocol"
	badgerState "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/blocktimer"
	"github.com/onflow/flow-go/storage/badger"
)

type VerificationConfig struct {
	chunkAlpha uint // number of verifiers assigned per chunk.
	chunkLimit uint // size of chunk-related memory pools.

	requestInterval    time.Duration // time interval that requester engine tries requesting chunk data packs.
	backoffMinInterval time.Duration // minimum time interval a chunk data pack request waits before dispatching.
	backoffMaxInterval time.Duration // maximum time interval a chunk data pack request waits before dispatching.
	backoffMultiplier  float64       // base of exponent in exponential backoff multiplier for backing off requests for chunk data packs.
	requestTargets     uint64        // maximum number of execution nodes a chunk data pack request is dispatched to.

	blockWorkers uint64 // number of blocks processed in parallel.
	chunkWorkers uint64 // number of chunks processed in parallel.

	stopAtHeight uint64 // height to stop the node on
}

type VerificationNodeBuilder struct {
	*FlowNodeBuilder
	verConf VerificationConfig
}

func NewVerificationNodeBuilder(nodeBuilder *FlowNodeBuilder) *VerificationNodeBuilder {
	return &VerificationNodeBuilder{
		FlowNodeBuilder: nodeBuilder,
		verConf:         VerificationConfig{},
	}
}

func (v *VerificationNodeBuilder) LoadFlags() {
	v.FlowNodeBuilder.
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.UintVar(&v.verConf.chunkLimit, "chunk-limit", 10000, "maximum number of chunk states in the memory pool")
			flags.UintVar(&v.verConf.chunkAlpha, "chunk-alpha", flow.DefaultChunkAssignmentAlpha, "number of verifiers should be assigned to each chunk")
			flags.DurationVar(&v.verConf.requestInterval, "chunk-request-interval", requester.DefaultRequestInterval, "time interval chunk data pack request is processed")
			flags.DurationVar(&v.verConf.backoffMinInterval, "backoff-min-interval", requester.DefaultBackoffMinInterval, "min time interval a chunk data pack request waits before dispatching")
			flags.DurationVar(&v.verConf.backoffMaxInterval, "backoff-max-interval", requester.DefaultBackoffMaxInterval, "min time interval a chunk data pack request waits before dispatching")
			flags.Float64Var(&v.verConf.backoffMultiplier, "backoff-multiplier", requester.DefaultBackoffMultiplier, "base of exponent in exponential backoff requesting mechanism")
			flags.Uint64Var(&v.verConf.requestTargets, "request-targets", requester.DefaultRequestTargets, "maximum number of execution nodes a chunk data pack request is dispatched to")
			flags.Uint64Var(&v.verConf.blockWorkers, "block-workers", blockconsumer.DefaultBlockWorkers, "maximum number of blocks being processed in parallel")
			flags.Uint64Var(&v.verConf.chunkWorkers, "chunk-workers", chunkconsumer.DefaultChunkWorkers, "maximum number of execution nodes a chunk data pack request is dispatched to")
			flags.Uint64Var(&v.verConf.stopAtHeight, "stop-at-height", 0, "height to stop the node at (0 to disable)")
		})
}

func (v *VerificationNodeBuilder) LoadComponentsAndModules() {
	var (
		followerState protocol.FollowerState

		chunkStatuses        *stdmap.ChunkStatuses    // used in fetcher engine
		chunkRequests        *stdmap.ChunkRequests    // used in requester engine
		processedChunkIndex  *badger.ConsumerProgress // used in chunk consumer
		processedBlockHeight *badger.ConsumerProgress // used in block consumer
		chunkQueue           *badger.ChunksQueue      // used in chunk consumer

		syncCore            *chainsync.Core   // used in follower engine
		assignerEngine      *assigner.Engine  // the assigner engine
		fetcherEngine       *fetcher.Engine   // the fetcher engine
		requesterEngine     *requester.Engine // the requester engine
		verifierEng         *verifier.Engine  // the verifier engine
		chunkConsumer       *chunkconsumer.ChunkConsumer
		blockConsumer       *blockconsumer.BlockConsumer
		followerDistributor *pubsub.FollowerDistributor

		committee    *committees.Consensus
		followerCore *hotstuff.FollowerLoop     // follower hotstuff logic
		followerEng  *follower.ComplianceEngine // the follower engine
		collector    module.VerificationMetrics // used to collect metrics of all engines
	)

	v.FlowNodeBuilder.
		PreInit(DynamicStartPreInit).
		Module("mutable follower state", func(node *NodeConfig) error {
			var err error
			// For now, we only support state implementations from package badger.
			// If we ever support different implementations, the following can be replaced by a type-aware factory
			state, ok := node.State.(*badgerState.State)
			if !ok {
				return fmt.Errorf("only implementations of type badger.State are currently supported but read-only state has type %T", node.State)
			}
			followerState, err = badgerState.NewFollowerState(
				node.Logger,
				node.Tracer,
				node.ProtocolEvents,
				state,
				node.Storage.Index,
				node.Storage.Payloads,
				blocktimer.DefaultBlockTimer,
			)
			return err
		}).
		Module("verification metrics", func(node *NodeConfig) error {
			collector = metrics.NewVerificationCollector(node.Tracer, node.MetricsRegisterer)
			return nil
		}).
		Module("chunk status memory pool", func(node *NodeConfig) error {
			var err error

			chunkStatuses = stdmap.NewChunkStatuses(v.verConf.chunkLimit)
			err = node.Metrics.Mempool.Register(metrics.ResourceChunkStatus, chunkStatuses.Size)
			if err != nil {
				return fmt.Errorf("could not register backend metric: %w", err)
			}
			return nil
		}).
		Module("chunk requests memory pool", func(node *NodeConfig) error {
			var err error

			chunkRequests = stdmap.NewChunkRequests(v.verConf.chunkLimit)
			err = node.Metrics.Mempool.Register(metrics.ResourceChunkRequest, chunkRequests.Size)
			if err != nil {
				return fmt.Errorf("could not register backend metric: %w", err)
			}
			return nil
		}).
		Module("processed chunk index consumer progress", func(node *NodeConfig) error {
			processedChunkIndex = badger.NewConsumerProgress(node.DB, module.ConsumeProgressVerificationChunkIndex)
			return nil
		}).
		Module("processed block height consumer progress", func(node *NodeConfig) error {
			processedBlockHeight = badger.NewConsumerProgress(node.DB, module.ConsumeProgressVerificationBlockHeight)
			return nil
		}).
		Module("chunks queue", func(node *NodeConfig) error {
			chunkQueue = badger.NewChunkQueue(node.DB)
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
		Module("follower distributor", func(node *NodeConfig) error {
			followerDistributor = pubsub.NewFollowerDistributor()
			followerDistributor.AddProposalViolationConsumer(notifications.NewSlashingViolationsConsumer(node.Logger))
			return nil
		}).
		Module("sync core", func(node *NodeConfig) error {
			var err error

			syncCore, err = chainsync.New(node.Logger, node.SyncCoreConfig, metrics.NewChainSyncCollector(node.RootChainID), node.RootChainID)
			return err
		}).
		Component("verifier engine", func(node *NodeConfig) (module.ReadyDoneAware, error) {
			var err error

			vm := fvm.NewVirtualMachine()
			fvmOptions := append(
				[]fvm.Option{fvm.WithLogger(node.Logger)},
				node.FvmOptions...,
			)
			vmCtx := fvm.NewContext(fvmOptions...)
			chunkVerifier := chunks.NewChunkVerifier(vm, vmCtx, node.Logger)
			approvalStorage := badger.NewResultApprovals(node.Metrics.Cache, node.DB)
			verifierEng, err = verifier.New(
				node.Logger,
				collector,
				node.Tracer,
				node.EngineRegistry,
				node.State,
				node.Me,
				chunkVerifier,
				approvalStorage)
			return verifierEng, err
		}).
		Component("chunk consumer, requester, and fetcher engines", func(node *NodeConfig) (module.ReadyDoneAware, error) {
			var err error

			requesterEngine, err = requester.New(
				node.Logger,
				node.State,
				node.EngineRegistry,
				node.Tracer,
				collector,
				chunkRequests,
				v.verConf.requestInterval,
				requester.RetryAfterQualifier,
				mempool.ExponentialUpdater(
					v.verConf.backoffMultiplier,
					v.verConf.backoffMaxInterval,
					v.verConf.backoffMinInterval,
				),
				v.verConf.requestTargets)
			if err != nil {
				return nil, fmt.Errorf("could not create requester engine: %w", err)
			}

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
				requesterEngine,
				v.verConf.stopAtHeight)

			// requester and fetcher engines are started by chunk consumer
			chunkConsumer, err = chunkconsumer.NewChunkConsumer(
				node.Logger,
				collector,
				processedChunkIndex,
				chunkQueue,
				fetcherEngine,
				v.verConf.chunkWorkers)

			if err != nil {
				return nil, fmt.Errorf("could not create chunk consumer: %w", err)
			}

			err = node.Metrics.Mempool.Register(metrics.ResourceChunkConsumer, chunkConsumer.Size)
			if err != nil {
				return nil, fmt.Errorf("could not register backend metric: %w", err)
			}

			return chunkConsumer, nil
		}).
		Component("assigner engine", func(node *NodeConfig) (module.ReadyDoneAware, error) {
			var chunkAssigner module.ChunkAssigner
			var err error

			chunkAssigner, err = chunks.NewChunkAssigner(v.verConf.chunkAlpha, node.State)
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
				chunkConsumer,
				v.verConf.stopAtHeight)

			return assignerEngine, nil
		}).
		Component("block consumer", func(node *NodeConfig) (module.ReadyDoneAware, error) {
			var initBlockHeight uint64
			var err error

			blockConsumer, initBlockHeight, err = blockconsumer.NewBlockConsumer(
				node.Logger,
				collector,
				processedBlockHeight,
				node.Storage.Blocks,
				node.State,
				assignerEngine,
				v.verConf.blockWorkers)

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
		Component("consensus committee", func(node *NodeConfig) (module.ReadyDoneAware, error) {
			// initialize consensus committee's membership state
			// This committee state is for the HotStuff follower, which follows the MAIN CONSENSUS Committee
			// Note: node.Me.NodeID() is not part of the consensus committee
			var err error
			committee, err = committees.NewConsensusCommittee(node.State, node.Me.NodeID())
			node.ProtocolEvents.AddConsumer(committee)
			return committee, err
		}).
		Component("follower core", func(node *NodeConfig) (module.ReadyDoneAware, error) {
			// create a finalizer that handles updating the protocol
			// state when the follower detects newly finalized blocks
			final := finalizer.NewFinalizer(node.DB, node.Storage.Headers, followerState, node.Tracer)

			finalized, pending, err := recoveryprotocol.FindLatest(node.State, node.Storage.Headers)
			if err != nil {
				return nil, fmt.Errorf("could not find latest finalized block and pending blocks to recover consensus follower: %w", err)
			}

			followerDistributor.AddOnBlockFinalizedConsumer(blockConsumer.OnFinalizedBlock)

			// creates a consensus follower with ingestEngine as the notifier
			// so that it gets notified upon each new finalized block
			followerCore, err = flowconsensus.NewFollower(
				node.Logger,
				node.Metrics.Mempool,
				node.Storage.Headers,
				final,
				followerDistributor,
				node.FinalizedRootBlock.Header,
				node.RootQC,
				finalized,
				pending,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create follower core logic: %w", err)
			}

			return followerCore, nil
		}).
		Component("follower engine", func(node *NodeConfig) (module.ReadyDoneAware, error) {
			packer := hotsignature.NewConsensusSigDataPacker(committee)
			// initialize the verifier for the protocol consensus
			verifier := verification.NewCombinedVerifier(committee, packer)
			validator := validator.New(committee, verifier)

			var heroCacheCollector module.HeroCacheMetrics = metrics.NewNoopCollector()
			if node.HeroCacheMetricsEnable {
				heroCacheCollector = metrics.FollowerCacheMetrics(node.MetricsRegisterer)
			}

			core, err := followereng.NewComplianceCore(
				node.Logger,
				node.Metrics.Mempool,
				heroCacheCollector,
				followerDistributor,
				followerState,
				followerCore,
				validator,
				syncCore,
				node.Tracer,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create follower core: %w", err)
			}

			followerEng, err = followereng.NewComplianceLayer(
				node.Logger,
				node.EngineRegistry,
				node.Me,
				node.Metrics.Engine,
				node.Storage.Headers,
				node.LastFinalizedHeader,
				core,
				node.ComplianceConfig,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create follower engine: %w", err)
			}
			followerDistributor.AddOnBlockFinalizedConsumer(followerEng.OnFinalizedBlock)

			return followerEng, nil
		}).
		Component("sync engine", func(node *NodeConfig) (module.ReadyDoneAware, error) {
			spamConfig, err := commonsync.NewSpamDetectionConfig()
			if err != nil {
				return nil, fmt.Errorf("could not initialize spam detection config: %w", err)
			}

			sync, err := commonsync.New(
				node.Logger,
				node.Metrics.Engine,
				node.EngineRegistry,
				node.Me,
				node.State,
				node.Storage.Blocks,
				followerEng,
				syncCore,
				node.SyncEngineIdentifierProvider,
				spamConfig,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create synchronization engine: %w", err)
			}
			followerDistributor.AddFinalizationConsumer(sync)

			return sync, nil
		})
}
