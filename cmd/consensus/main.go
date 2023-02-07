// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/pflag"

	client "github.com/onflow/flow-go-sdk/access/grpc"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/blockproducer"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker/timeout"
	"github.com/onflow/flow-go/consensus/hotstuff/persister"
	hotsignature "github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/consensus/hotstuff/timeoutcollector"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/consensus/hotstuff/votecollector"
	recovery "github.com/onflow/flow-go/consensus/recovery/protocol"
	"github.com/onflow/flow-go/engine/common/requester"
	synceng "github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/engine/consensus/approvals/tracker"
	"github.com/onflow/flow-go/engine/consensus/compliance"
	dkgeng "github.com/onflow/flow-go/engine/consensus/dkg"
	"github.com/onflow/flow-go/engine/consensus/ingestion"
	"github.com/onflow/flow-go/engine/consensus/matching"
	"github.com/onflow/flow-go/engine/consensus/message_hub"
	"github.com/onflow/flow-go/engine/consensus/sealing"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/buffer"
	builder "github.com/onflow/flow-go/module/builder/consensus"
	"github.com/onflow/flow-go/module/chainsync"
	chmodule "github.com/onflow/flow-go/module/chunks"
	modulecompliance "github.com/onflow/flow-go/module/compliance"
	dkgmodule "github.com/onflow/flow-go/module/dkg"
	"github.com/onflow/flow-go/module/epochs"
	finalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/mempool"
	consensusMempools "github.com/onflow/flow-go/module/mempool/consensus"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	msig "github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/module/updatable_configs"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/module/validation"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/state/protocol"
	badgerState "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/blocktimer"
	"github.com/onflow/flow-go/state/protocol/events/gadgets"
	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/io"
)

func main() {

	var (
		guaranteeLimit                       uint
		resultLimit                          uint
		approvalLimit                        uint
		sealLimit                            uint
		pendingReceiptsLimit                 uint
		minInterval                          time.Duration
		maxInterval                          time.Duration
		maxSealPerBlock                      uint
		maxGuaranteePerBlock                 uint
		hotstuffMinTimeout                   time.Duration
		hotstuffTimeoutAdjustmentFactor      float64
		hotstuffHappyPathMaxRoundFailures    uint64
		blockRateDelay                       time.Duration
		chunkAlpha                           uint
		requiredApprovalsForSealVerification uint
		requiredApprovalsForSealConstruction uint
		emergencySealing                     bool
		dkgControllerConfig                  dkgmodule.ControllerConfig
		startupTimeString                    string
		startupTime                          time.Time

		// DKG contract client
		machineAccountInfo *bootstrap.NodeMachineAccountInfo
		flowClientConfigs  []*common.FlowClientConfig
		insecureAccessAPI  bool
		accessNodeIDS      []string

		err                     error
		mutableState            protocol.MutableState
		beaconPrivateKey        *encodable.RandomBeaconPrivKey
		guarantees              mempool.Guarantees
		receipts                mempool.ExecutionTree
		seals                   mempool.IncorporatedResultSeals
		pendingReceipts         mempool.PendingReceipts
		receiptRequester        *requester.Engine
		syncCore                *chainsync.Core
		comp                    *compliance.Engine
		hot                     module.HotStuff
		conMetrics              module.ConsensusMetrics
		mainMetrics             module.HotstuffMetrics
		receiptValidator        module.ReceiptValidator
		chunkAssigner           *chmodule.ChunkAssigner
		finalizationDistributor *pubsub.FinalizationDistributor
		dkgBrokerTunnel         *dkgmodule.BrokerTunnel
		blockTimer              protocol.BlockTimer
		finalizedHeader         *synceng.FinalizedHeaderCache
		committee               *committees.Consensus
		epochLookup             *epochs.EpochLookup
		hotstuffModules         *consensus.HotstuffModules
		dkgState                *bstorage.DKGState
		safeBeaconKeys          *bstorage.SafeBeaconPrivateKeys
		getSealingConfigs       module.SealingConfigsGetter
	)

	nodeBuilder := cmd.FlowNode(flow.RoleConsensus.String())
	nodeBuilder.ExtraFlags(func(flags *pflag.FlagSet) {
		flags.UintVar(&guaranteeLimit, "guarantee-limit", 1000, "maximum number of guarantees in the memory pool")
		flags.UintVar(&resultLimit, "result-limit", 10000, "maximum number of execution results in the memory pool")
		flags.UintVar(&approvalLimit, "approval-limit", 1000, "maximum number of result approvals in the memory pool")
		// the default value is able to buffer as many seals as would be generated over ~12 hours. In case it
		// ever gets full, the node will simply crash instead of employing complex ejection logic.
		flags.UintVar(&sealLimit, "seal-limit", 44200, "maximum number of block seals in the memory pool")
		flags.UintVar(&pendingReceiptsLimit, "pending-receipts-limit", 10000, "maximum number of pending receipts in the mempool")
		flags.DurationVar(&minInterval, "min-interval", time.Millisecond, "the minimum amount of time between two blocks")
		flags.DurationVar(&maxInterval, "max-interval", 90*time.Second, "the maximum amount of time between two blocks")
		flags.UintVar(&maxSealPerBlock, "max-seal-per-block", 100, "the maximum number of seals to be included in a block")
		flags.UintVar(&maxGuaranteePerBlock, "max-guarantee-per-block", 100, "the maximum number of collection guarantees to be included in a block")
		flags.DurationVar(&hotstuffMinTimeout, "hotstuff-min-timeout", 2500*time.Millisecond, "the lower timeout bound for the hotstuff pacemaker, this is also used as initial timeout")
		flags.Float64Var(&hotstuffTimeoutAdjustmentFactor, "hotstuff-timeout-adjustment-factor", timeout.DefaultConfig.TimeoutAdjustmentFactor, "adjustment of timeout duration in case of time out event")
		flags.Uint64Var(&hotstuffHappyPathMaxRoundFailures, "hotstuff-happy-path-max-round-failures", timeout.DefaultConfig.HappyPathMaxRoundFailures, "number of failed rounds before first timeout increase")
		flags.DurationVar(&blockRateDelay, "block-rate-delay", 500*time.Millisecond, "the delay to broadcast block proposal in order to control block production rate")
		flags.UintVar(&chunkAlpha, "chunk-alpha", flow.DefaultChunkAssignmentAlpha, "number of verifiers that should be assigned to each chunk")
		flags.UintVar(&requiredApprovalsForSealVerification, "required-verification-seal-approvals", flow.DefaultRequiredApprovalsForSealValidation, "minimum number of approvals that are required to verify a seal")
		flags.UintVar(&requiredApprovalsForSealConstruction, "required-construction-seal-approvals", flow.DefaultRequiredApprovalsForSealConstruction, "minimum number of approvals that are required to construct a seal")
		flags.BoolVar(&emergencySealing, "emergency-sealing-active", flow.DefaultEmergencySealingActive, "(de)activation of emergency sealing")
		flags.BoolVar(&insecureAccessAPI, "insecure-access-api", false, "required if insecure GRPC connection should be used")
		flags.StringSliceVar(&accessNodeIDS, "access-node-ids", []string{}, fmt.Sprintf("array of access node IDs sorted in priority order where the first ID in this array will get the first connection attempt and each subsequent ID after serves as a fallback. Minimum length %d. Use '*' for all IDs in protocol state.", common.DefaultAccessNodeIDSMinimum))
		flags.DurationVar(&dkgControllerConfig.BaseStartDelay, "dkg-controller-base-start-delay", dkgmodule.DefaultBaseStartDelay, "used to define the range for jitter prior to DKG start (eg. 500µs) - the base value is scaled quadratically with the # of DKG participants")
		flags.DurationVar(&dkgControllerConfig.BaseHandleFirstBroadcastDelay, "dkg-controller-base-handle-first-broadcast-delay", dkgmodule.DefaultBaseHandleFirstBroadcastDelay, "used to define the range for jitter prior to DKG handling the first broadcast messages (eg. 50ms) - the base value is scaled quadratically with the # of DKG participants")
		flags.DurationVar(&dkgControllerConfig.HandleSubsequentBroadcastDelay, "dkg-controller-handle-subsequent-broadcast-delay", dkgmodule.DefaultHandleSubsequentBroadcastDelay, "used to define the constant delay introduced prior to DKG handling subsequent broadcast messages (eg. 2s)")
		flags.StringVar(&startupTimeString, "hotstuff-startup-time", cmd.NotSet, "specifies date and time (in ISO 8601 format) after which the consensus participant may enter the first view (e.g 1996-04-24T15:04:05-07:00)")
	}).ValidateFlags(func() error {
		nodeBuilder.Logger.Info().Str("startup_time_str", startupTimeString).Msg("got startup_time_str")
		if startupTimeString != cmd.NotSet {
			t, err := time.Parse(time.RFC3339, startupTimeString)
			if err != nil {
				return fmt.Errorf("invalid start-time value: %w", err)
			}
			startupTime = t
			nodeBuilder.Logger.Info().Time("startup_time", startupTime).Msg("got startup_time")
		}
		return nil
	})

	if err = nodeBuilder.Initialize(); err != nil {
		nodeBuilder.Logger.Fatal().Err(err).Send()
	}

	nodeBuilder.
		PreInit(cmd.DynamicStartPreInit).
		ValidateRootSnapshot(badgerState.ValidRootSnapshotContainsEntityExpiryRange).
		Module("consensus node metrics", func(node *cmd.NodeConfig) error {
			conMetrics = metrics.NewConsensusCollector(node.Tracer, node.MetricsRegisterer)
			return nil
		}).
		Module("dkg state", func(node *cmd.NodeConfig) error {
			dkgState, err = bstorage.NewDKGState(node.Metrics.Cache, node.SecretsDB)
			return err
		}).
		Module("beacon keys", func(node *cmd.NodeConfig) error {
			safeBeaconKeys = bstorage.NewSafeBeaconPrivateKeys(dkgState)
			return nil
		}).
		Module("updatable sealing config", func(node *cmd.NodeConfig) error {
			setter, err := updatable_configs.NewSealingConfigs(
				requiredApprovalsForSealConstruction,
				requiredApprovalsForSealVerification,
				chunkAlpha,
				emergencySealing,
			)
			if err != nil {
				return err
			}

			// update the getter with the setter, so other modules can only get, but not set
			getSealingConfigs = setter

			// admin tool is the only instance that have access to the setter interface, therefore, is
			// the only module can change this config
			err = node.ConfigManager.RegisterUintConfig("consensus-required-approvals-for-sealing",
				setter.RequireApprovalsForSealConstructionDynamicValue,
				setter.SetRequiredApprovalsForSealingConstruction)
			return err
		}).
		Module("mutable follower state", func(node *cmd.NodeConfig) error {
			// For now, we only support state implementations from package badger.
			// If we ever support different implementations, the following can be replaced by a type-aware factory
			state, ok := node.State.(*badgerState.State)
			if !ok {
				return fmt.Errorf("only implementations of type badger.State are currently supported but read-only state has type %T", node.State)
			}

			chunkAssigner, err = chmodule.NewChunkAssigner(chunkAlpha, node.State)
			if err != nil {
				return fmt.Errorf("could not instantiate assignment algorithm for chunk verification: %w", err)
			}

			receiptValidator = validation.NewReceiptValidator(
				node.State,
				node.Storage.Headers,
				node.Storage.Index,
				node.Storage.Results,
				node.Storage.Seals)

			sealValidator := validation.NewSealValidator(
				node.State,
				node.Storage.Headers,
				node.Storage.Index,
				node.Storage.Results,
				node.Storage.Seals,
				chunkAssigner,
				getSealingConfigs,
				conMetrics)

			blockTimer, err = blocktimer.NewBlockTimer(minInterval, maxInterval)
			if err != nil {
				return err
			}

			mutableState, err = badgerState.NewFullConsensusState(
				state,
				node.Storage.Index,
				node.Storage.Payloads,
				node.Tracer,
				node.ProtocolEvents,
				blockTimer,
				receiptValidator,
				sealValidator)
			return err
		}).
		Module("random beacon key", func(node *cmd.NodeConfig) error {
			// If this node was a participant in a spork, their beacon key for the
			// first epoch was generated during the bootstrapping process and is
			// specified in a private bootstrapping file. We load their key and
			// store it in the db for the initial post-spork epoch for use going
			// forward.
			//
			// If this node was not a participant in a spork, they joined at an
			// epoch boundary, so they have no beacon key file (they will generate
			// their first beacon private key through the DKG in the EpochSetup phase
			// prior to their first epoch as network participant).

			rootSnapshot := node.State.AtBlockID(node.RootBlock.ID())
			isSporkRoot, err := protocol.IsSporkRootSnapshot(rootSnapshot)
			if err != nil {
				return fmt.Errorf("could not check whether root snapshot is spork root: %w", err)
			}
			if !isSporkRoot {
				node.Logger.Info().Msg("node starting from mid-spork snapshot, will not read spork random beacon key file")
				return nil
			}

			// If the node has a beacon key file, then save it to the secrets database
			// as the beacon key for the epoch of the root snapshot.
			beaconPrivateKey, err = loadBeaconPrivateKey(node.BaseConfig.BootstrapDir, node.NodeID)
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("node is starting from spork root snapshot, but does not have spork random beacon key file: %w", err)
			}
			if err != nil {
				return fmt.Errorf("could not load beacon key file: %w", err)
			}

			rootEpoch := node.State.AtBlockID(node.RootBlock.ID()).Epochs().Current()
			epochCounter, err := rootEpoch.Counter()
			if err != nil {
				return fmt.Errorf("could not get root epoch counter: %w", err)
			}

			// confirm the beacon key file matches the canonical public keys
			rootDKG, err := rootEpoch.DKG()
			if err != nil {
				return fmt.Errorf("could not get dkg for root epoch: %w", err)
			}
			myBeaconPublicKeyShare, err := rootDKG.KeyShare(node.NodeID)
			if err != nil {
				return fmt.Errorf("could not get my beacon public key share for root epoch: %w", err)
			}

			if !myBeaconPublicKeyShare.Equals(beaconPrivateKey.PrivateKey.PublicKey()) {
				return fmt.Errorf("configured beacon key is inconsistent with this node's canonical public beacon key (%s!=%s)",
					beaconPrivateKey.PrivateKey.PublicKey(),
					myBeaconPublicKeyShare)
			}

			// store my beacon key for the first epoch post-spork
			err = dkgState.InsertMyBeaconPrivateKey(epochCounter, beaconPrivateKey.PrivateKey)
			if err != nil && !errors.Is(err, storage.ErrAlreadyExists) {
				return err
			}
			// mark the root DKG as successful, so it is considered safe to use the key
			err = dkgState.SetDKGEndState(epochCounter, flow.DKGEndStateSuccess)
			if err != nil && !errors.Is(err, storage.ErrAlreadyExists) {
				return err
			}

			return nil
		}).
		Module("collection guarantees mempool", func(node *cmd.NodeConfig) error {
			guarantees, err = stdmap.NewGuarantees(guaranteeLimit)
			return err
		}).
		Module("execution receipts mempool", func(node *cmd.NodeConfig) error {
			receipts = consensusMempools.NewExecutionTree()
			// registers size method of backend for metrics
			err = node.Metrics.Mempool.Register(metrics.ResourceReceipt, receipts.Size)
			if err != nil {
				return fmt.Errorf("could not register backend metric: %w", err)
			}
			return nil
		}).
		Module("block seals mempool", func(node *cmd.NodeConfig) error {
			// use a custom ejector, so we don't eject seals that would break
			// the chain of seals
			rawMempool := stdmap.NewIncorporatedResultSeals(sealLimit)
			multipleReceiptsFilterMempool := consensusMempools.NewIncorporatedResultSeals(rawMempool, node.Storage.Receipts)
			seals, err = consensusMempools.NewExecStateForkSuppressor(
				multipleReceiptsFilterMempool,
				consensusMempools.LogForkAndCrash(node.Logger),
				node.DB,
				node.Logger,
			)
			if err != nil {
				return fmt.Errorf("failed to wrap seals mempool into ExecStateForkSuppressor: %w", err)
			}
			err = node.Metrics.Mempool.Register(metrics.ResourcePendingIncorporatedSeal, seals.Size)
			return nil
		}).
		Module("pending receipts mempool", func(node *cmd.NodeConfig) error {
			pendingReceipts = stdmap.NewPendingReceipts(node.Storage.Headers, pendingReceiptsLimit)
			return nil
		}).
		Module("hotstuff main metrics", func(node *cmd.NodeConfig) error {
			mainMetrics = metrics.NewHotstuffCollector(node.RootChainID)
			return nil
		}).
		Module("sync core", func(node *cmd.NodeConfig) error {
			syncCore, err = chainsync.New(node.Logger, node.SyncCoreConfig, metrics.NewChainSyncCollector(node.RootChainID), node.RootChainID)
			return err
		}).
		Module("finalization distributor", func(node *cmd.NodeConfig) error {
			finalizationDistributor = pubsub.NewFinalizationDistributor()
			return nil
		}).
		Module("machine account config", func(node *cmd.NodeConfig) error {
			machineAccountInfo, err = cmd.LoadNodeMachineAccountInfoFile(node.BootstrapDir, node.NodeID)
			return err
		}).
		Module("sdk client connection options", func(node *cmd.NodeConfig) error {
			anIDS, err := common.ValidateAccessNodeIDSFlag(accessNodeIDS, node.RootChainID, node.State.Sealed())
			if err != nil {
				return fmt.Errorf("failed to validate flag --access-node-ids %w", err)
			}

			flowClientConfigs, err = common.FlowClientConfigs(anIDS, insecureAccessAPI, node.State.Sealed())
			if err != nil {
				return fmt.Errorf("failed to prepare flow client connection configs for each access node id %w", err)
			}

			return nil
		}).
		Component("machine account config validator", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			//@TODO use fallback logic for flowClient similar to DKG/QC contract clients
			flowClient, err := common.FlowClient(flowClientConfigs[0])
			if err != nil {
				return nil, fmt.Errorf("failed to get flow client connection option for access node (0): %s %w", flowClientConfigs[0].AccessAddress, err)
			}

			// disable balance checks for transient networks, which do not have transaction fees
			var opts []epochs.MachineAccountValidatorConfigOption
			if node.RootChainID.Transient() {
				opts = append(opts, epochs.WithoutBalanceChecks)
			}
			validator, err := epochs.NewMachineAccountConfigValidator(
				node.Logger,
				flowClient,
				flow.RoleCollection,
				*machineAccountInfo,
				opts...,
			)
			return validator, err
		}).
		Component("sealing engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {

			sealingTracker := tracker.NewSealingTracker(node.Logger, node.Storage.Headers, node.Storage.Receipts, seals)

			e, err := sealing.NewEngine(
				node.Logger,
				node.Tracer,
				conMetrics,
				node.Metrics.Engine,
				node.Metrics.Mempool,
				sealingTracker,
				node.Network,
				node.Me,
				node.Storage.Headers,
				node.Storage.Payloads,
				node.Storage.Results,
				node.Storage.Index,
				node.State,
				node.Storage.Seals,
				chunkAssigner,
				seals,
				getSealingConfigs,
			)

			// subscribe for finalization events from hotstuff
			finalizationDistributor.AddOnBlockFinalizedConsumer(e.OnFinalizedBlock)
			finalizationDistributor.AddOnBlockIncorporatedConsumer(e.OnBlockIncorporated)

			return e, err
		}).
		Component("matching engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			receiptRequester, err = requester.New(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.Me,
				node.State,
				channels.RequestReceiptsByBlockID,
				filter.HasRole(flow.RoleExecution),
				func() flow.Entity { return &flow.ExecutionReceipt{} },
				requester.WithRetryInitial(2*time.Second),
				requester.WithRetryMaximum(30*time.Second),
			)
			if err != nil {
				return nil, err
			}

			core := matching.NewCore(
				node.Logger,
				node.Tracer,
				conMetrics,
				node.Metrics.Mempool,
				node.State,
				node.Storage.Headers,
				node.Storage.Receipts,
				receipts,
				pendingReceipts,
				seals,
				receiptValidator,
				receiptRequester,
				matching.DefaultConfig(),
			)

			e, err := matching.NewEngine(
				node.Logger,
				node.Network,
				node.Me,
				node.Metrics.Engine,
				node.Metrics.Mempool,
				node.State,
				node.Storage.Receipts,
				node.Storage.Index,
				core,
			)
			if err != nil {
				return nil, err
			}

			// subscribe engine to inputs from other node-internal components
			receiptRequester.WithHandle(e.HandleReceipt)
			finalizationDistributor.AddOnBlockFinalizedConsumer(e.OnFinalizedBlock)
			finalizationDistributor.AddOnBlockIncorporatedConsumer(e.OnBlockIncorporated)

			return e, err
		}).
		Component("ingestion engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			core := ingestion.NewCore(
				node.Logger,
				node.Tracer,
				node.Metrics.Mempool,
				node.State,
				node.Storage.Headers,
				guarantees,
			)

			ing, err := ingestion.New(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.Me,
				core,
			)

			return ing, err
		}).
		Component("hotstuff committee", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			committee, err = committees.NewConsensusCommittee(node.State, node.Me.NodeID())
			node.ProtocolEvents.AddConsumer(committee)
			return committee, err
		}).
		Component("epoch lookup", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			epochLookup, err = epochs.NewEpochLookup(node.State)
			node.ProtocolEvents.AddConsumer(epochLookup)
			return epochLookup, err
		}).
		Component("hotstuff modules", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			// initialize the block finalizer
			finalize := finalizer.NewFinalizer(
				node.DB,
				node.Storage.Headers,
				mutableState,
				node.Tracer,
				finalizer.WithCleanup(finalizer.CleanupMempools(
					node.Metrics.Mempool,
					conMetrics,
					node.Storage.Payloads,
					guarantees,
					seals,
				)),
			)

			// wrap Main consensus committee with metrics
			wrappedCommittee := committees.NewMetricsWrapper(committee, mainMetrics) // wrapper for measuring time spent determining consensus committee relations

			beaconKeyStore := hotsignature.NewEpochAwareRandomBeaconKeyStore(epochLookup, safeBeaconKeys)

			// initialize the combined signer for hotstuff
			var signer hotstuff.Signer
			signer = verification.NewCombinedSigner(
				node.Me,
				beaconKeyStore,
			)
			signer = verification.NewMetricsWrapper(signer, mainMetrics) // wrapper for measuring time spent with crypto-related operations

			// create consensus logger
			logger := createLogger(node.Logger, node.RootChainID)

			// initialize a logging notifier for hotstuff
			notifier := createNotifier(
				logger,
				mainMetrics,
			)

			notifier.AddConsumer(finalizationDistributor)

			// initialize the persister
			persist := persister.New(node.DB, node.RootChainID)

			finalizedBlock, err := node.State.Final().Head()
			if err != nil {
				return nil, err
			}

			forks, err := consensus.NewForks(
				finalizedBlock,
				node.Storage.Headers,
				finalize,
				notifier,
				node.RootBlock.Header,
				node.RootQC,
			)
			if err != nil {
				return nil, err
			}

			qcDistributor := pubsub.NewQCCreatedDistributor()
			validator := consensus.NewValidator(mainMetrics, wrappedCommittee)
			voteProcessorFactory := votecollector.NewCombinedVoteProcessorFactory(wrappedCommittee, qcDistributor.OnQcConstructedFromVotes)
			lowestViewForVoteProcessing := finalizedBlock.View + 1
			voteAggregator, err := consensus.NewVoteAggregator(
				logger,
				mainMetrics,
				node.Metrics.Engine,
				node.Metrics.Mempool,
				lowestViewForVoteProcessing,
				notifier,
				voteProcessorFactory,
				finalizationDistributor)
			if err != nil {
				return nil, fmt.Errorf("could not initialize vote aggregator: %w", err)
			}

			timeoutCollectorDistributor := pubsub.NewTimeoutCollectorDistributor()
			timeoutProcessorFactory := timeoutcollector.NewTimeoutProcessorFactory(
				logger,
				timeoutCollectorDistributor,
				committee,
				validator,
				msig.ConsensusTimeoutTag,
			)
			timeoutAggregator, err := consensus.NewTimeoutAggregator(
				logger,
				mainMetrics,
				node.Metrics.Engine,
				node.Metrics.Mempool,
				notifier,
				timeoutProcessorFactory,
				timeoutCollectorDistributor,
				lowestViewForVoteProcessing,
			)
			if err != nil {
				return nil, fmt.Errorf("could not initialize timeout aggregator: %w", err)
			}

			hotstuffModules = &consensus.HotstuffModules{
				Notifier:                    notifier,
				Committee:                   wrappedCommittee,
				Signer:                      signer,
				Persist:                     persist,
				QCCreatedDistributor:        qcDistributor,
				FinalizationDistributor:     finalizationDistributor,
				TimeoutCollectorDistributor: timeoutCollectorDistributor,
				Forks:                       forks,
				Validator:                   validator,
				VoteAggregator:              voteAggregator,
				TimeoutAggregator:           timeoutAggregator,
			}

			return util.MergeReadyDone(voteAggregator, timeoutAggregator), nil
		}).
		Component("consensus participant", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			// initialize the block builder
			var build module.Builder
			build, err = builder.NewBuilder(
				node.Metrics.Mempool,
				node.DB,
				mutableState,
				node.Storage.Headers,
				node.Storage.Seals,
				node.Storage.Index,
				node.Storage.Blocks,
				node.Storage.Results,
				node.Storage.Receipts,
				guarantees,
				seals,
				receipts,
				node.Tracer,
				builder.WithBlockTimer(blockTimer),
				builder.WithMaxSealCount(maxSealPerBlock),
				builder.WithMaxGuaranteeCount(maxGuaranteePerBlock),
			)
			if err != nil {
				return nil, fmt.Errorf("could not initialized block builder: %w", err)
			}
			build = blockproducer.NewMetricsWrapper(build, mainMetrics) // wrapper for measuring time spent building block payload component

			opts := []consensus.Option{
				consensus.WithMinTimeout(hotstuffMinTimeout),
				consensus.WithTimeoutAdjustmentFactor(hotstuffTimeoutAdjustmentFactor),
				consensus.WithHappyPathMaxRoundFailures(hotstuffHappyPathMaxRoundFailures),
				consensus.WithBlockRateDelay(blockRateDelay),
				consensus.WithConfigRegistrar(node.ConfigManager),
			}

			if !startupTime.IsZero() {
				opts = append(opts, consensus.WithStartupTime(startupTime))
			}
			finalizedBlock, pending, err := recovery.FindLatest(node.State, node.Storage.Headers)
			if err != nil {
				return nil, err
			}

			// initialize hotstuff consensus algorithm
			hot, err = consensus.NewParticipant(
				createLogger(node.Logger, node.RootChainID),
				mainMetrics,
				build,
				finalizedBlock,
				pending,
				hotstuffModules,
				opts...,
			)
			if err != nil {
				return nil, fmt.Errorf("could not initialize hotstuff engine: %w", err)
			}
			return hot, nil
		}).
		Component("consensus compliance engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			// initialize the entity database accessors
			cleaner := bstorage.NewCleaner(node.Logger, node.DB, node.Metrics.CleanCollector, flow.DefaultValueLogGCFrequency)

			// initialize the pending blocks cache
			proposals := buffer.NewPendingBlocks()

			logger := createLogger(node.Logger, node.RootChainID)
			complianceCore, err := compliance.NewCore(logger,
				node.Metrics.Engine,
				node.Metrics.Mempool,
				mainMetrics,
				node.Metrics.Compliance,
				node.Tracer,
				cleaner,
				node.Storage.Headers,
				node.Storage.Payloads,
				mutableState,
				proposals,
				syncCore,
				hotstuffModules.Validator,
				hot,
				hotstuffModules.VoteAggregator,
				hotstuffModules.TimeoutAggregator,
				modulecompliance.WithSkipNewProposalsThreshold(node.ComplianceConfig.SkipNewProposalsThreshold),
			)
			if err != nil {
				return nil, fmt.Errorf("could not initialize compliance core: %w", err)
			}

			// initialize the compliance engine
			comp, err = compliance.NewEngine(
				logger,
				node.Me,
				complianceCore,
			)
			if err != nil {
				return nil, fmt.Errorf("could not initialize compliance engine: %w", err)
			}

			finalizationDistributor.AddOnBlockFinalizedConsumer(comp.OnFinalizedBlock)

			return comp, nil
		}).
		Component("consensus message hub", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			messageHub, err := message_hub.NewMessageHub(
				createLogger(node.Logger, node.RootChainID),
				node.Metrics.Engine,
				node.Network,
				node.Me,
				comp,
				hot,
				hotstuffModules.VoteAggregator,
				hotstuffModules.TimeoutAggregator,
				node.State,
				node.Storage.Payloads,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create consensus message hub: %w", err)
			}
			hotstuffModules.Notifier.AddConsumer(messageHub)
			return messageHub, nil
		}).
		Component("finalized snapshot", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			finalizedHeader, err = synceng.NewFinalizedHeaderCache(node.Logger, node.State, finalizationDistributor)
			if err != nil {
				return nil, fmt.Errorf("could not create finalized snapshot cache: %w", err)
			}

			return finalizedHeader, nil
		}).
		Component("sync engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			sync, err := synceng.New(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.Me,
				node.Storage.Blocks,
				comp,
				syncCore,
				finalizedHeader,
				node.SyncEngineIdentifierProvider,
			)
			if err != nil {
				return nil, fmt.Errorf("could not initialize synchronization engine: %w", err)
			}

			return sync, nil
		}).
		Component("receipt requester engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			// created with sealing engine
			return receiptRequester, nil
		}).
		Component("DKG messaging engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {

			// brokerTunnel is used to forward messages between the DKG
			// messaging engine and the DKG broker/controller
			dkgBrokerTunnel = dkgmodule.NewBrokerTunnel()

			// messagingEngine is a network engine that is used by nodes to
			// exchange private DKG messages
			messagingEngine, err := dkgeng.NewMessagingEngine(
				node.Logger,
				node.Network,
				node.Me,
				dkgBrokerTunnel,
			)
			if err != nil {
				return nil, fmt.Errorf("could not initialize DKG messaging engine: %w", err)
			}

			return messagingEngine, nil
		}).
		Component("DKG reactor engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			// the viewsObserver is used by the reactor engine to subscribe to
			// new views being finalized
			viewsObserver := gadgets.NewViews()
			node.ProtocolEvents.AddConsumer(viewsObserver)

			// construct DKG contract client
			dkgContractClients, err := createDKGContractClients(node, machineAccountInfo, flowClientConfigs)
			if err != nil {
				return nil, fmt.Errorf("could not create dkg contract client %w", err)
			}

			// the reactor engine reacts to new views being finalized and drives the
			// DKG protocol
			reactorEngine := dkgeng.NewReactorEngine(
				node.Logger,
				node.Me,
				node.State,
				dkgState,
				dkgmodule.NewControllerFactory(
					node.Logger,
					node.Me,
					dkgContractClients,
					dkgBrokerTunnel,
					dkgControllerConfig,
				),
				viewsObserver,
			)

			// reactorEngine consumes the EpochSetupPhaseStarted event
			node.ProtocolEvents.AddConsumer(reactorEngine)

			return reactorEngine, nil
		})

	node, err := nodeBuilder.Build()
	if err != nil {
		nodeBuilder.Logger.Fatal().Err(err).Send()
	}
	node.Run()
}

func loadBeaconPrivateKey(dir string, myID flow.Identifier) (*encodable.RandomBeaconPrivKey, error) {
	path := fmt.Sprintf(bootstrap.PathRandomBeaconPriv, myID)
	data, err := io.ReadFile(filepath.Join(dir, path))
	if err != nil {
		return nil, err
	}

	var priv encodable.RandomBeaconPrivKey
	err = json.Unmarshal(data, &priv)
	if err != nil {
		return nil, err
	}
	return &priv, nil
}

// createDKGContractClient creates an dkgContractClient
func createDKGContractClient(node *cmd.NodeConfig, machineAccountInfo *bootstrap.NodeMachineAccountInfo, flowClient *client.Client, anID flow.Identifier) (module.DKGContractClient, error) {
	var dkgClient module.DKGContractClient

	contracts, err := systemcontracts.SystemContractsForChain(node.RootChainID)
	if err != nil {
		return nil, err
	}
	dkgContractAddress := contracts.DKG.Address.Hex()

	// construct signer from private key
	sk, err := crypto.DecodePrivateKey(machineAccountInfo.SigningAlgorithm, machineAccountInfo.EncodedPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("could not decode private key from hex: %w", err)
	}

	txSigner, err := crypto.NewInMemorySigner(sk, machineAccountInfo.HashAlgorithm)
	if err != nil {
		return nil, fmt.Errorf("could not create in-memory signer: %w", err)
	}

	// create actual dkg contract client, all flags and machine account info file found
	dkgClient = dkgmodule.NewClient(
		node.Logger,
		flowClient,
		anID,
		txSigner,
		dkgContractAddress,
		machineAccountInfo.Address,
		machineAccountInfo.KeyIndex,
	)

	return dkgClient, nil
}

// createDKGContractClients creates an array dkgContractClient that is sorted by retry fallback priority
func createDKGContractClients(node *cmd.NodeConfig, machineAccountInfo *bootstrap.NodeMachineAccountInfo, flowClientOpts []*common.FlowClientConfig) ([]module.DKGContractClient, error) {
	dkgClients := make([]module.DKGContractClient, 0)

	for _, opt := range flowClientOpts {
		flowClient, err := common.FlowClient(opt)
		if err != nil {
			return nil, fmt.Errorf("failed to create flow client for dkg contract client with options: %s %w", flowClientOpts, err)
		}

		node.Logger.Info().Msgf("created dkg contract client with opts: %s", opt.String())
		dkgClient, err := createDKGContractClient(node, machineAccountInfo, flowClient, opt.AccessNodeID)
		if err != nil {
			return nil, fmt.Errorf("failed to create dkg contract client with flow client options: %s %w", flowClientOpts, err)
		}

		dkgClients = append(dkgClients, dkgClient)
	}

	return dkgClients, nil
}
