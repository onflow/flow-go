package main

import (
	"fmt"
	"time"

	"github.com/onflow/flow-go/cmd/util/cmd/common"

	"github.com/spf13/pflag"

	"github.com/onflow/flow-go-sdk/client"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker/timeout"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	recovery "github.com/onflow/flow-go/consensus/recovery/protocol"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/collection/epochmgr"
	"github.com/onflow/flow-go/engine/collection/epochmgr/factories"
	"github.com/onflow/flow-go/engine/collection/ingest"
	"github.com/onflow/flow-go/engine/collection/pusher"
	followereng "github.com/onflow/flow-go/engine/common/follower"
	"github.com/onflow/flow-go/engine/common/provider"
	consync "github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/buffer"
	builder "github.com/onflow/flow-go/module/builder/collection"
	"github.com/onflow/flow-go/module/epochs"
	confinalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/ingress"
	"github.com/onflow/flow-go/module/mempool"
	epochpool "github.com/onflow/flow-go/module/mempool/epochs"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/state/protocol"
	badgerState "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/blocktimer"
	"github.com/onflow/flow-go/state/protocol/events/gadgets"
	storagekv "github.com/onflow/flow-go/storage/badger"
)

func main() {

	var (
		txLimit                                uint
		maxCollectionSize                      uint
		maxCollectionByteSize                  uint64
		maxCollectionTotalGas                  uint64
		builderExpiryBuffer                    uint
		builderPayerRateLimit                  float64
		builderUnlimitedPayers                 []string
		hotstuffTimeout                        time.Duration
		hotstuffMinTimeout                     time.Duration
		hotstuffTimeoutIncreaseFactor          float64
		hotstuffTimeoutDecreaseFactor          float64
		hotstuffTimeoutVoteAggregationFraction float64
		blockRateDelay                         time.Duration

		followerState protocol.MutableState
		ingestConf    ingest.Config
		ingressConf   ingress.Config

		pools                   *epochpool.TransactionPools // epoch-scoped transaction pools
		followerBuffer          *buffer.PendingBlocks       // pending block cache for follower
		finalizationDistributor *pubsub.FinalizationDistributor
		finalizedHeader         *consync.FinalizedHeaderCache

		push              *pusher.Engine
		ing               *ingest.Engine
		mainChainSyncCore *synchronization.Core
		followerEng       *followereng.Engine
		colMetrics        module.CollectionMetrics
		err               error

		// epoch qc contract client
		accessAddress      string
		secureAccessNodeID string

		insecureAccessAPI bool
	)

	nodeBuilder := cmd.FlowNode(flow.RoleCollection.String())
	nodeBuilder.ExtraFlags(func(flags *pflag.FlagSet) {
		flags.UintVar(&txLimit, "tx-limit", 50000,
			"maximum number of transactions in the memory pool")
		flags.StringVarP(&ingressConf.ListenAddr, "ingress-addr", "i", "localhost:9000",
			"the address the ingress server listens on")
		flags.BoolVar(&ingressConf.RpcMetricsEnabled, "rpc-metrics-enabled", false,
			"whether to enable the rpc metrics")
		flags.Uint64Var(&ingestConf.MaxGasLimit, "ingest-max-gas-limit", flow.DefaultMaxTransactionGasLimit,
			"maximum per-transaction computation limit (gas limit)")
		flags.Uint64Var(&ingestConf.MaxTransactionByteSize, "ingest-max-tx-byte-size", flow.DefaultMaxTransactionByteSize,
			"maximum per-transaction byte size")
		flags.Uint64Var(&ingestConf.MaxCollectionByteSize, "ingest-max-col-byte-size", flow.DefaultMaxCollectionByteSize,
			"maximum per-collection byte size")
		flags.BoolVar(&ingestConf.CheckScriptsParse, "ingest-check-scripts-parse", true,
			"whether we check that inbound transactions are parse-able")
		flags.UintVar(&ingestConf.ExpiryBuffer, "ingest-expiry-buffer", 30,
			"expiry buffer for inbound transactions")
		flags.UintVar(&ingestConf.PropagationRedundancy, "ingest-tx-propagation-redundancy", 10,
			"how many additional cluster members we propagate transactions to")
		flags.Uint64Var(&ingestConf.MaxAddressIndex, "ingest-max-address-index", 10_000_000,
			"the maximum address index allowed in transactions")
		flags.UintVar(&builderExpiryBuffer, "builder-expiry-buffer", builder.DefaultExpiryBuffer,
			"expiry buffer for transactions in proposed collections")
		flags.Float64Var(&builderPayerRateLimit, "builder-rate-limit", builder.DefaultMaxPayerTransactionRate, // no rate limiting
			"rate limit for each payer (transactions/collection)")
		flags.StringSliceVar(&builderUnlimitedPayers, "builder-unlimited-payers", []string{}, // no unlimited payers
			"set of payer addresses which are omitted from rate limiting")
		flags.UintVar(&maxCollectionSize, "builder-max-collection-size", flow.DefaultMaxCollectionSize,
			"maximum number of transactions in proposed collections")
		flags.Uint64Var(&maxCollectionByteSize, "builder-max-collection-byte-size", flow.DefaultMaxCollectionByteSize,
			"maximum byte size of the proposed collection")
		flags.Uint64Var(&maxCollectionTotalGas, "builder-max-collection-total-gas", flow.DefaultMaxCollectionTotalGas,
			"maximum total amount of maxgas of transactions in proposed collections")
		flags.DurationVar(&hotstuffTimeout, "hotstuff-timeout", 60*time.Second,
			"the initial timeout for the hotstuff pacemaker")
		flags.DurationVar(&hotstuffMinTimeout, "hotstuff-min-timeout", 2500*time.Millisecond,
			"the lower timeout bound for the hotstuff pacemaker")
		flags.Float64Var(&hotstuffTimeoutIncreaseFactor, "hotstuff-timeout-increase-factor",
			timeout.DefaultConfig.TimeoutIncrease,
			"multiplicative increase of timeout value in case of time out event")
		flags.Float64Var(&hotstuffTimeoutDecreaseFactor, "hotstuff-timeout-decrease-factor",
			timeout.DefaultConfig.TimeoutDecrease,
			"multiplicative decrease of timeout value in case of progress")
		flags.Float64Var(&hotstuffTimeoutVoteAggregationFraction, "hotstuff-timeout-vote-aggregation-fraction",
			timeout.DefaultConfig.VoteAggregationTimeoutFraction,
			"additional fraction of replica timeout that the primary will wait for votes")
		flags.DurationVar(&blockRateDelay, "block-rate-delay", 250*time.Millisecond,
			"the delay to broadcast block proposal in order to control block production rate")

		// epoch qc contract flags
		flags.StringVar(&accessAddress, "access-address", "", "the address of an access node")
		flags.StringVar(&secureAccessNodeID, "secure-access-node-id", "", "the node ID of the secure access GRPC server")
		flags.BoolVar(&insecureAccessAPI, "insecure-access-api", true, "required if insecure GRPC connection should be used")
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
		Module("transactions mempool", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			create := func() mempool.Transactions { return stdmap.NewTransactions(txLimit) }
			pools = epochpool.NewTransactionPools(create)
			err := node.Metrics.Mempool.Register(metrics.ResourceTransaction, pools.CombinedSize)
			return err
		}).
		Module("pending block cache", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			followerBuffer = buffer.NewPendingBlocks()
			return nil
		}).
		Module("metrics", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			colMetrics = metrics.NewCollectionCollector(node.Tracer)
			return nil
		}).
		Module("main chain sync core", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			mainChainSyncCore, err = synchronization.New(node.Logger, synchronization.DefaultConfig())
			return err
		}).
		Component("follower engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {

			// initialize cleaner for DB
			cleaner := storagekv.NewCleaner(node.Logger, node.DB, node.Metrics.CleanCollector, flow.DefaultValueLogGCFrequency)

			// create a finalizer that will handling updating the protocol
			// state when the follower detects newly finalized blocks
			finalizer := confinalizer.NewFinalizer(node.DB, node.Storage.Headers, followerState)

			// initialize the staking & beacon verifiers, signature joiner
			staking := signature.NewAggregationVerifier(encoding.ConsensusVoteTag)
			beacon := signature.NewThresholdVerifier(encoding.RandomBeaconTag)
			merger := signature.NewCombiner(encodable.ConsensusVoteSigLen, encodable.RandomBeaconSigLen)

			// initialize consensus committee's membership state
			// This committee state is for the HotStuff follower, which follows the MAIN CONSENSUS Committee
			// Note: node.Me.NodeID() is not part of the consensus committee
			mainConsensusCommittee, err := committees.NewConsensusCommittee(node.State, node.Me.NodeID())
			if err != nil {
				return nil, fmt.Errorf("could not create Committee state for main consensus: %w", err)
			}

			// initialize the verifier for the protocol consensus
			verifier := verification.NewCombinedVerifier(mainConsensusCommittee, staking, beacon, merger)

			finalizationDistributor = pubsub.NewFinalizationDistributor()

			finalized, pending, err := recovery.FindLatest(node.State, node.Storage.Headers)
			if err != nil {
				return nil, fmt.Errorf("could not find latest finalized block and pending blocks to recover consensus follower: %w", err)
			}

			// creates a consensus follower with noop consumer as the notifier
			followerCore, err := consensus.NewFollower(
				node.Logger,
				mainConsensusCommittee,
				node.Storage.Headers,
				finalizer,
				verifier,
				finalizationDistributor,
				node.RootBlock.Header,
				node.RootQC,
				finalized,
				pending,
			)
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
				followerBuffer,
				followerCore,
				mainChainSyncCore,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create follower engine: %w", err)
			}

			return followerEng, nil
		}).
		Component("finalized snapshot", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			finalizedHeader, err = consync.NewFinalizedHeaderCache(node.Logger, node.State, finalizationDistributor)
			if err != nil {
				return nil, fmt.Errorf("could not create finalized snapshot cache: %w", err)
			}

			return finalizedHeader, nil
		}).
		Component("main chain sync engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {

			// create a block synchronization engine to handle follower getting out of sync
			sync, err := consync.New(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.Me,
				node.Storage.Blocks,
				followerEng,
				mainChainSyncCore,
				finalizedHeader,
				node.SyncEngineIdentifierProvider,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create synchronization engine: %w", err)
			}

			return sync, nil
		}).
		Component("ingestion engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			ing, err = ingest.New(
				node.Logger,
				node.Network,
				node.State,
				node.Metrics.Engine,
				colMetrics,
				node.Me,
				node.RootChainID.Chain(),
				pools,
				ingestConf,
			)
			return ing, err
		}).
		Component("transaction ingress server", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			server := ingress.New(ingressConf, ing, node.RootChainID)
			return server, nil
		}).
		Component("provider engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			retrieve := func(collID flow.Identifier) (flow.Entity, error) {
				coll, err := node.Storage.Collections.ByID(collID)
				return coll, err
			}
			return provider.New(node.Logger, node.Metrics.Engine, node.Network, node.Me, node.State,
				engine.ProvideCollections,
				filter.HasRole(flow.RoleAccess, flow.RoleExecution),
				retrieve,
			)
		}).
		Component("pusher engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			push, err = pusher.New(
				node.Logger,
				node.Network,
				node.State,
				node.Metrics.Engine,
				colMetrics,
				node.Me,
				node.Storage.Collections,
				node.Storage.Transactions,
			)
			return push, err
		}).
		// Epoch manager encapsulates and manages epoch-dependent engines as we
		// transition between epochs
		Component("epoch manager", func(_ cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			clusterStateFactory, err := factories.NewClusterStateFactory(node.DB, node.Metrics.Cache, node.Tracer)
			if err != nil {
				return nil, err
			}

			// convert hex string flag values to addresses
			unlimitedPayers := make([]flow.Address, 0, len(builderUnlimitedPayers))
			for _, payerStr := range builderUnlimitedPayers {
				payerAddr := flow.HexToAddress(payerStr)
				unlimitedPayers = append(unlimitedPayers, payerAddr)
			}

			builderFactory, err := factories.NewBuilderFactory(
				node.DB,
				node.Storage.Headers,
				node.Tracer,
				colMetrics,
				push,
				builder.WithMaxCollectionSize(maxCollectionSize),
				builder.WithMaxCollectionByteSize(maxCollectionByteSize),
				builder.WithMaxCollectionTotalGas(maxCollectionTotalGas),
				builder.WithExpiryBuffer(builderExpiryBuffer),
				builder.WithMaxPayerTransactionRate(builderPayerRateLimit),
				builder.WithUnlimitedPayers(unlimitedPayers...),
			)
			if err != nil {
				return nil, err
			}

			proposalFactory, err := factories.NewProposalEngineFactory(
				node.Logger,
				node.Network,
				node.Me,
				colMetrics,
				node.Metrics.Engine,
				node.Metrics.Mempool,
				node.State,
				node.Storage.Transactions,
			)
			if err != nil {
				return nil, err
			}

			syncFactory, err := factories.NewSyncEngineFactory(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.Me,
				synchronization.DefaultConfig(),
			)
			if err != nil {
				return nil, err
			}

			createMetrics := func(chainID flow.ChainID) module.HotstuffMetrics {
				return metrics.NewHotstuffCollector(chainID)
			}
			staking := signature.NewAggregationProvider(encoding.CollectorVoteTag, node.Me)
			hotstuffFactory, err := factories.NewHotStuffFactory(
				node.Logger,
				node.Me,
				staking,
				node.DB,
				node.State,
				createMetrics,
				consensus.WithBlockRateDelay(blockRateDelay),
				consensus.WithInitialTimeout(hotstuffTimeout),
				consensus.WithMinTimeout(hotstuffMinTimeout),
				consensus.WithVoteAggregationTimeoutFraction(hotstuffTimeoutVoteAggregationFraction),
				consensus.WithTimeoutIncreaseFactor(hotstuffTimeoutIncreaseFactor),
				consensus.WithTimeoutDecreaseFactor(hotstuffTimeoutDecreaseFactor),
			)
			if err != nil {
				return nil, err
			}

			signer := verification.NewSingleSigner(staking, node.Me.NodeID())

			// create flow client with correct GRPC configuration for QC contract client
			var flowClient *client.Client
			if insecureAccessAPI {
				flowClient, err = common.InsecureFlowClient(accessAddress)
				if err != nil {
					return nil, err
				}
			} else {
				if secureAccessNodeID == "" {
					return nil, fmt.Errorf("invalid flag --secure-access-node-id required")
				}

				nodeID, err := flow.HexStringToIdentifier(secureAccessNodeID)
				if err != nil {
					return nil, fmt.Errorf("could not get flow identifier from secured access node id: %s", secureAccessNodeID)
				}

				identities, err := node.State.Sealed().Identities(filter.HasNodeID(nodeID))
				if err != nil {
					return nil, fmt.Errorf("could not get identity of secure access node: %s", secureAccessNodeID)
				}

				if len(identities) < 1 {
					return nil, fmt.Errorf("could not find identity of secure access node: %s", secureAccessNodeID)
				}

				flowClient, err = common.SecureFlowClient(accessAddress, identities[0].NetworkPubKey.String()[2:])
				if err != nil {
					return nil, err
				}
			}

			// construct QC contract client
			qcContractClient, err := createQCContractClient(node, accessAddress, flowClient)
			if err != nil {
				return nil, fmt.Errorf("could not create qc contract client %w", err)
			}

			rootQCVoter := epochs.NewRootQCVoter(
				node.Logger,
				node.Me,
				signer,
				node.State,
				qcContractClient,
			)

			factory := factories.NewEpochComponentsFactory(
				node.Me,
				pools,
				builderFactory,
				clusterStateFactory,
				hotstuffFactory,
				proposalFactory,
				syncFactory,
			)

			heightEvents := gadgets.NewHeights()
			node.ProtocolEvents.AddConsumer(heightEvents)

			manager, err := epochmgr.New(
				node.Logger,
				node.Me,
				node.State,
				pools,
				rootQCVoter,
				factory,
				heightEvents,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create epoch manager: %w", err)
			}

			// register the manager for protocol events
			node.ProtocolEvents.AddConsumer(manager)

			return manager, err
		}).
		Run()
}

// createQCContractClient creates QC contract client
func createQCContractClient(node *cmd.NodeConfig, accessAddress string, flowClient *client.Client) (module.QCContractClient, error) {

	var qcContractClient module.QCContractClient

	contracts, err := systemcontracts.SystemContractsForChain(node.RootChainID)
	if err != nil {
		return nil, err
	}
	qcContractAddress := contracts.ClusterQC.Address.Hex()

	// if not valid return a mock qc contract client
	if valid := cmd.IsValidNodeMachineAccountConfig(node, accessAddress); !valid {
		return nil, fmt.Errorf("could not validate node machine account config")
	}

	// attempt to read NodeMachineAccountInfo
	info, err := cmd.LoadNodeMachineAccountInfoFile(node.BaseConfig.BootstrapDir, node.Me.NodeID())
	if err != nil {
		return nil, fmt.Errorf("could not load node machine account info file: %w", err)
	}

	// construct signer from private key
	sk, err := sdkcrypto.DecodePrivateKey(info.SigningAlgorithm, info.EncodedPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("could not decode private key from hex: %w", err)
	}
	txSigner := sdkcrypto.NewInMemorySigner(sk, info.HashAlgorithm)

	// create actual qc contract client, all flags and machine account info file found
	qcContractClient = epochs.NewQCContractClient(node.Logger, flowClient, node.Me.NodeID(), info.Address, info.KeyIndex, qcContractAddress, txSigner)

	return qcContractClient, nil
}
