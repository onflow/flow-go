package integration_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/gammazero/workerpool"
	"github.com/onflow/crypto"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	bootstrapDKG "github.com/onflow/flow-go/cmd/bootstrap/dkg"
	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/consensus/hotstuff/persister"
	hsig "github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/consensus/hotstuff/timeoutaggregator"
	"github.com/onflow/flow-go/consensus/hotstuff/timeoutcollector"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/consensus/hotstuff/voteaggregator"
	"github.com/onflow/flow-go/consensus/hotstuff/votecollector"
	synceng "github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/engine/consensus/compliance"
	"github.com/onflow/flow-go/engine/consensus/message_hub"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/buffer"
	builder "github.com/onflow/flow-go/module/builder/consensus"
	synccore "github.com/onflow/flow-go/module/chainsync"
	modulecompliance "github.com/onflow/flow-go/module/compliance"
	finalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/local"
	consensusMempools "github.com/onflow/flow-go/module/mempool/consensus"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	msig "github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol"
	bprotocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/blocktimer"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/state/protocol/protocol_state/kvstore"
	protocol_state "github.com/onflow/flow-go/state/protocol/protocol_state/state"
	"github.com/onflow/flow-go/state/protocol/util"
	fstorage "github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

const hotstuffTimeout = 500 * time.Millisecond

// RandomBeaconNodeInfo stores information about participation in DKG process for consensus node
// contains private + public keys and participant index
// Each node has unique structure
type RandomBeaconNodeInfo struct {
	RandomBeaconPrivKey crypto.PrivateKey
	DKGParticipant      flow.DKGParticipant
}

// ConsensusParticipant stores information about node which is fixed during epoch changes
// like staking key, role, network key and random beacon info which changes every epoch
// Contains a mapping of DKG info per epoch.
type ConsensusParticipant struct {
	nodeInfo          bootstrap.NodeInfo
	beaconInfoByEpoch map[uint64]RandomBeaconNodeInfo
}

// ConsensusParticipants is a special cache which stores information about consensus participants across multiple epochs
// This structure is used to launch nodes in our integration test setup
type ConsensusParticipants struct {
	lookup map[flow.Identifier]ConsensusParticipant // nodeID -> ConsensusParticipant
}

func NewConsensusParticipants(data *run.ParticipantData) *ConsensusParticipants {
	lookup := make(map[flow.Identifier]ConsensusParticipant)
	for _, participant := range data.Participants {
		lookup[participant.NodeID] = ConsensusParticipant{
			nodeInfo: participant.NodeInfo,
			beaconInfoByEpoch: map[uint64]RandomBeaconNodeInfo{
				1: {
					RandomBeaconPrivKey: participant.RandomBeaconPrivKey,
					DKGParticipant:      data.DKGCommittee[participant.NodeID],
				},
			},
		}
	}
	return &ConsensusParticipants{
		lookup: lookup,
	}
}

// Lookup performs lookup of participant by nodeID
func (p *ConsensusParticipants) Lookup(nodeID flow.Identifier) *ConsensusParticipant {
	participant, ok := p.lookup[nodeID]
	if ok {
		return &participant
	}
	return nil
}

// Update stores information about consensus participants for some epoch
// If this node was part of previous epoch it will get updated, if not created.
func (p *ConsensusParticipants) Update(epochCounter uint64, data *run.ParticipantData) {
	for _, participant := range data.Participants {
		dkgParticipant := data.DKGCommittee[participant.NodeID]
		entry, ok := p.lookup[participant.NodeID]
		if !ok {
			entry = ConsensusParticipant{
				nodeInfo:          participant.NodeInfo,
				beaconInfoByEpoch: map[uint64]RandomBeaconNodeInfo{},
			}
		}

		entry.beaconInfoByEpoch[epochCounter] = RandomBeaconNodeInfo{
			RandomBeaconPrivKey: participant.RandomBeaconPrivKey,
			DKGParticipant:      dkgParticipant,
		}
		p.lookup[participant.NodeID] = entry
	}
}

type Node struct {
	db                fstorage.DB
	dbCloser          io.Closer
	dbDir             string
	index             int
	log               zerolog.Logger
	id                *flow.Identity
	compliance        *compliance.Engine
	sync              *synceng.Engine
	hot               module.HotStuff
	committee         *committees.Consensus
	voteAggregator    hotstuff.VoteAggregator
	timeoutAggregator hotstuff.TimeoutAggregator
	messageHub        *message_hub.MessageHub
	state             *bprotocol.ParticipantState
	headers           fstorage.Headers
	net               *Network
}

// epochInfo is a helper structure for storing epoch information such as counter and final view
type epochInfo struct {
	finalView uint64
	counter   uint64
}

// buildEpochLookupList is a helper function which builds an auxiliary structure of epochs sorted by counter
func buildEpochLookupList(epochs ...protocol.CommittedEpoch) []epochInfo {
	infos := make([]epochInfo, 0)
	for _, epoch := range epochs {
		infos = append(infos, epochInfo{
			finalView: epoch.FinalView(),
			counter:   epoch.Counter(),
		})
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].finalView < infos[j].finalView
	})
	return infos
}

// createNodes creates consensus nodes based on the input ConsensusParticipants info.
// All nodes will be started using a common parent context.
// Each node is connected to the Stopper, which will cancel the context when the
// stopping condition is reached.
// The list of created nodes, the common network hub, and a function which starts
// all the nodes together, is returned.
func createNodes(t *testing.T, participants *ConsensusParticipants, rootSnapshot protocol.Snapshot, stopper *Stopper) (nodes []*Node, hub *Hub, runFor func(time.Duration)) {
	consensus, err := rootSnapshot.Identities(filter.HasRole[flow.Identity](flow.RoleConsensus))
	require.NoError(t, err)

	var epochViewLookup []epochInfo
	currentEpoch, err := rootSnapshot.Epochs().Current()
	require.NoError(t, err)
	// Whether there is a next committed epoch depends on the test.
	nextEpoch, err := rootSnapshot.Epochs().NextCommitted()
	if err != nil { // the only acceptable error here is `protocol.ErrNextEpochNotCommitted`
		require.ErrorIs(t, err, protocol.ErrNextEpochNotCommitted)
		epochViewLookup = buildEpochLookupList(currentEpoch)
	} else {
		epochViewLookup = buildEpochLookupList(currentEpoch, nextEpoch)
	}

	epochLookup := &mockmodule.EpochLookup{}
	epochLookup.On("EpochForView", mock.Anything).Return(
		func(view uint64) uint64 {
			for _, info := range epochViewLookup {
				if view <= info.finalView {
					return info.counter
				}
			}
			return 0
		}, func(view uint64) error {
			if view > epochViewLookup[len(epochViewLookup)-1].finalView {
				return fmt.Errorf("unexpected epoch transition")
			} else {
				return nil
			}
		})

	hub = NewNetworkHub()
	nodes = make([]*Node, 0, len(consensus))
	for i, identity := range consensus {
		consensusParticipant := participants.Lookup(identity.NodeID)
		require.NotNil(t, consensusParticipant)
		node := createNode(t, consensusParticipant, i, identity, rootSnapshot, hub, stopper, epochLookup)
		nodes = append(nodes, node)
	}

	// create a context which will be used for all nodes
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	// create a function to return which the test case can use to run the nodes for some maximum duration
	// and gracefully stop after.
	runFor = func(maxDuration time.Duration) {
		runNodes(signalerCtx, nodes)
		unittest.RequireCloseBefore(t, stopper.stopped, maxDuration, "expect to get signal from stopper before timeout")
		stopNodes(t, cancel, nodes)
	}

	stopper.WithStopFunc(func() {

	})

	return nodes, hub, runFor
}

func createRootQC(t *testing.T, root *flow.Block, participantData *run.ParticipantData) *flow.QuorumCertificate {
	consensusCluster := participantData.Identities()
	votes, err := run.GenerateRootBlockVotes(root, participantData)
	require.NoError(t, err)
	qc, invalidVotes, err := run.GenerateRootQC(root, votes, participantData, consensusCluster)
	require.NoError(t, err)
	require.Len(t, invalidVotes, 0)
	return qc
}

// createRootBlockData creates genesis block with first epoch and real data node identities.
// This function requires all participants to pass DKG process.
func createRootBlockData(t *testing.T, participantData *run.ParticipantData) (*flow.Block, *flow.ExecutionResult, *flow.Seal) {
	rootHeaderBody := unittest.Block.Genesis(flow.Emulator).HeaderBody
	consensusParticipants := participantData.Identities()

	// add other roles to create a complete identity list
	participants := unittest.CompleteIdentitySet(consensusParticipants...).Sort(flow.Canonical[flow.Identity])
	dkgParticipants := participants.ToSkeleton().Filter(filter.IsConsensusCommitteeMember)
	dkgParticipantsKeys := make([]crypto.PublicKey, 0, len(consensusParticipants))
	dkgIndexMap := make(flow.DKGIndexMap)
	for index, participant := range dkgParticipants {
		dkgParticipantsKeys = append(dkgParticipantsKeys, participantData.DKGCommittee[participant.NodeID].KeyShare)
		dkgIndexMap[participant.NodeID] = index
	}

	counter := uint64(1)
	setup := unittest.EpochSetupFixture(
		unittest.WithParticipants(participants.ToSkeleton()),
		unittest.SetupWithCounter(counter),
		unittest.WithFirstView(rootHeaderBody.View),
		unittest.WithFinalView(rootHeaderBody.View+1000),
	)
	commit := unittest.EpochCommitFixture(
		unittest.CommitWithCounter(counter),
		unittest.WithClusterQCsFromAssignments(setup.Assignments),
		func(commit *flow.EpochCommit) {
			commit.DKGGroupKey = participantData.DKGGroupKey
			commit.DKGParticipantKeys = dkgParticipantsKeys
			commit.DKGIndexMap = dkgIndexMap
		},
	)
	minEpochStateEntry, err := inmem.EpochProtocolStateFromServiceEvents(setup, commit)
	require.NoError(t, err)
	epochProtocolStateID := minEpochStateEntry.ID()
	safetyParams, err := protocol.DefaultEpochSafetyParams(rootHeaderBody.ChainID)
	require.NoError(t, err)
	rootProtocolState, err := kvstore.NewDefaultKVStore(safetyParams.FinalizationSafetyThreshold, safetyParams.EpochExtensionViewCount, epochProtocolStateID)
	require.NoError(t, err)
	root, err := flow.NewRootBlock(
		flow.UntrustedBlock{
			HeaderBody: rootHeaderBody,
			Payload:    flow.Payload{ProtocolStateID: rootProtocolState.ID()},
		},
	)
	require.NoError(t, err)
	result := unittest.BootstrapExecutionResultFixture(root, unittest.GenesisStateCommitment)
	result.ServiceEvents = []flow.ServiceEvent{setup.ServiceEvent(), commit.ServiceEvent()}

	seal := unittest.Seal.Fixture(unittest.Seal.WithResult(result))

	return root, result, seal
}

func createPrivateNodeIdentities(t *testing.T, n int) []bootstrap.NodeInfo {
	consensus := unittest.IdentityListFixture(n, unittest.WithRole(flow.RoleConsensus)).Sort(flow.Canonical[flow.Identity])
	infos := make([]bootstrap.NodeInfo, 0, n)
	for _, node := range consensus {
		networkPrivKey := unittest.NetworkingPrivKeyFixture()
		stakingPrivKey := unittest.StakingPrivKeyFixture()
		nodeInfo, err := bootstrap.NewPrivateNodeInfo(
			node.NodeID,
			node.Role,
			node.Address,
			node.InitialWeight,
			networkPrivKey,
			stakingPrivKey,
		)
		require.NoError(t, err)
		infos = append(infos, nodeInfo)
	}
	return infos
}

func createConsensusIdentities(t *testing.T, n int) *run.ParticipantData {
	// create n consensus node participants
	consensus := createPrivateNodeIdentities(t, n)
	return completeConsensusIdentities(t, consensus)
}

// completeConsensusIdentities runs KG process and fills nodeInfos with missing random beacon keys
func completeConsensusIdentities(t *testing.T, nodeInfos []bootstrap.NodeInfo) *run.ParticipantData {
	dkgData, err := bootstrapDKG.RandomBeaconKG(len(nodeInfos), unittest.RandomBytes(48))
	require.NoError(t, err)

	participantData := &run.ParticipantData{
		Participants: make([]run.Participant, 0, len(nodeInfos)),
		DKGCommittee: make(map[flow.Identifier]flow.DKGParticipant),
		DKGGroupKey:  dkgData.PubGroupKey,
	}
	for index, node := range nodeInfos {
		participant := run.Participant{
			NodeInfo:            node,
			RandomBeaconPrivKey: dkgData.PrivKeyShares[index],
		}
		participantData.Participants = append(participantData.Participants, participant)
		participantData.DKGCommittee[node.NodeID] = flow.DKGParticipant{
			Index:    uint(index),
			KeyShare: dkgData.PubKeyShares[index],
		}
	}

	return participantData
}

// createRootSnapshot creates root block, generates root QC and builds a root snapshot for
// bootstrapping a node
func createRootSnapshot(t *testing.T, participantData *run.ParticipantData) *inmem.Snapshot {
	root, result, seal := createRootBlockData(t, participantData)
	rootQC := createRootQC(t, root, participantData)

	rootSnapshot, err := unittest.SnapshotFromBootstrapState(root, result, seal, rootQC)
	require.NoError(t, err)
	return rootSnapshot
}

func createNode(
	t *testing.T,
	participant *ConsensusParticipant,
	index int,
	identity *flow.Identity,
	rootSnapshot protocol.Snapshot,
	hub *Hub,
	stopper *Stopper,
	epochLookup module.EpochLookup,
) *Node {

	pdb, dbDir := unittest.TempPebbleDB(t)
	metricsCollector := metrics.NewNoopCollector()
	tracer := trace.NewNoopTracer()
	db := pebbleimpl.ToDB(pdb)
	lockManager := fstorage.NewTestingLockManager()

	headersDB := store.NewHeaders(metricsCollector, db)
	guaranteesDB := store.NewGuarantees(metricsCollector, db, store.DefaultCacheSize, store.DefaultCacheSize)
	sealsDB := store.NewSeals(metricsCollector, db)
	indexDB := store.NewIndex(metricsCollector, db)
	resultsDB := store.NewExecutionResults(metricsCollector, db)
	receiptsDB := store.NewExecutionReceipts(metricsCollector, db, resultsDB, store.DefaultCacheSize)
	payloadsDB := store.NewPayloads(db, indexDB, guaranteesDB, sealsDB, receiptsDB, resultsDB)
	blocksDB := store.NewBlocks(db, headersDB, payloadsDB)
	qcsDB := store.NewQuorumCertificates(metricsCollector, db, store.DefaultCacheSize)
	setupsDB := store.NewEpochSetups(metricsCollector, db)
	commitsDB := store.NewEpochCommits(metricsCollector, db)
	protocolStateDB := store.NewEpochProtocolStateEntries(metricsCollector, setupsDB, commitsDB, db,
		store.DefaultEpochProtocolStateCacheSize, store.DefaultProtocolStateIndexCacheSize)
	protocolKVStoreDB := store.NewProtocolKVStore(metricsCollector, db,
		store.DefaultProtocolKVStoreCacheSize, store.DefaultProtocolKVStoreByBlockIDCacheSize)
	versionBeaconDB := store.NewVersionBeacons(db)
	protocolStateEvents := events.NewDistributor()

	localNodeID := identity.NodeID

	log := unittest.Logger().With().
		Int("index", index).
		Hex("node_id", localNodeID[:]).
		Logger()

	state, err := bprotocol.Bootstrap(
		metricsCollector,
		db,
		lockManager,
		headersDB,
		sealsDB,
		resultsDB,
		blocksDB,
		qcsDB,
		setupsDB,
		commitsDB,
		protocolStateDB,
		protocolKVStoreDB,
		versionBeaconDB,
		rootSnapshot,
	)
	require.NoError(t, err)

	blockTimer, err := blocktimer.NewBlockTimer(1, 90_000)
	require.NoError(t, err)

	fullState, err := bprotocol.NewFullConsensusState(
		log,
		tracer,
		protocolStateEvents,
		state,
		indexDB,
		payloadsDB,
		blockTimer,
		util.MockReceiptValidator(),
		util.MockSealValidator(sealsDB),
	)
	require.NoError(t, err)

	node := &Node{
		db:    db,
		dbDir: dbDir,
		index: index,
		id:    identity,
	}

	stopper.AddNode(node)

	counterConsumer := &CounterConsumer{
		finalized: func(total uint) {
			stopper.onFinalizedTotal(node.id.NodeID, total)
		},
	}

	// log with node index
	logConsumer := notifications.NewLogConsumer(log)
	hotstuffDistributor := pubsub.NewDistributor()
	hotstuffDistributor.AddConsumer(counterConsumer)
	hotstuffDistributor.AddConsumer(logConsumer)

	require.Equal(t, participant.nodeInfo.NodeID, localNodeID)
	privateKeys, err := participant.nodeInfo.PrivateKeys()
	require.NoError(t, err)

	// make local
	me, err := local.New(identity.IdentitySkeleton, privateKeys.StakingKey)
	require.NoError(t, err)

	// add a network for this node to the hub
	net := hub.AddNetwork(localNodeID, node)

	guaranteeLimit, sealLimit := uint(1000), uint(1000)
	guarantees := stdmap.NewGuarantees(guaranteeLimit)

	receipts := consensusMempools.NewExecutionTree()

	seals := stdmap.NewIncorporatedResultSeals(sealLimit)

	mutableProtocolState := protocol_state.NewMutableProtocolState(
		log,
		protocolStateDB,
		protocolKVStoreDB,
		state.Params(),
		headersDB,
		resultsDB,
		setupsDB,
		commitsDB,
	)

	// initialize the block builder
	build, err := builder.NewBuilder(
		metricsCollector,
		fullState,
		headersDB,
		sealsDB,
		indexDB,
		blocksDB,
		resultsDB,
		receiptsDB,
		mutableProtocolState,
		guarantees,
		consensusMempools.NewIncorporatedResultSeals(seals, receiptsDB),
		receipts,
		tracer,
	)
	require.NoError(t, err)

	// initialize the pending blocks cache
	cache := buffer.NewPendingBlocks()

	rootHeader, err := rootSnapshot.Head()
	require.NoError(t, err)

	rootQC, err := rootSnapshot.QuorumCertificate()
	require.NoError(t, err)

	committee, err := committees.NewConsensusCommittee(state, localNodeID)
	require.NoError(t, err)
	protocolStateEvents.AddConsumer(committee)

	// initialize the block finalizer
	final := finalizer.NewFinalizer(db.Reader(), headersDB, fullState, trace.NewNoopTracer())

	syncCore, err := synccore.New(log, synccore.DefaultConfig(), metricsCollector, rootHeader.ChainID)
	require.NoError(t, err)

	voteAggregationDistributor := pubsub.NewVoteAggregationDistributor()
	voteAggregationDistributor.AddVoteAggregationConsumer(logConsumer)

	forks, err := consensus.NewForks(rootHeader, headersDB, final, hotstuffDistributor, rootHeader, rootQC)
	require.NoError(t, err)

	validator := consensus.NewValidator(metricsCollector, committee)
	require.NoError(t, err)

	keys := &storagemock.SafeBeaconKeys{}
	// there is Random Beacon key for this epoch
	keys.On("RetrieveMyBeaconPrivateKey", mock.Anything).Return(
		func(epochCounter uint64) crypto.PrivateKey {
			dkgInfo, ok := participant.beaconInfoByEpoch[epochCounter]
			if !ok {
				return nil
			}
			return dkgInfo.RandomBeaconPrivKey
		},
		func(epochCounter uint64) bool {
			_, ok := participant.beaconInfoByEpoch[epochCounter]
			return ok
		},
		nil)

	// use epoch aware store for testing scenarios where epoch changes
	beaconKeyStore := hsig.NewEpochAwareRandomBeaconKeyStore(epochLookup, keys)

	signer := verification.NewCombinedSigner(me, beaconKeyStore)

	persist, err := persister.New(db, rootHeader.ChainID)
	require.NoError(t, err)

	livenessData, err := persist.GetLivenessData()
	require.NoError(t, err)

	voteProcessorFactory := votecollector.NewCombinedVoteProcessorFactory(committee, voteAggregationDistributor.OnQcConstructedFromVotes)

	createCollectorFactoryMethod := votecollector.NewStateMachineFactory(log, voteAggregationDistributor, voteProcessorFactory.Create)
	voteCollectors := voteaggregator.NewVoteCollectors(log, livenessData.CurrentView, workerpool.New(2), createCollectorFactoryMethod)

	voteAggregator, err := voteaggregator.NewVoteAggregator(
		log,
		metricsCollector,
		metricsCollector,
		metricsCollector,
		voteAggregationDistributor,
		livenessData.CurrentView,
		voteCollectors,
	)
	require.NoError(t, err)

	timeoutAggregationDistributor := pubsub.NewTimeoutAggregationDistributor()
	timeoutAggregationDistributor.AddTimeoutCollectorConsumer(logConsumer)

	timeoutProcessorFactory := timeoutcollector.NewTimeoutProcessorFactory(
		log,
		timeoutAggregationDistributor,
		committee,
		validator,
		msig.ConsensusTimeoutTag,
	)
	timeoutCollectorsFactory := timeoutcollector.NewTimeoutCollectorFactory(
		log,
		timeoutAggregationDistributor,
		timeoutProcessorFactory,
	)
	timeoutCollectors := timeoutaggregator.NewTimeoutCollectors(
		log,
		metricsCollector,
		livenessData.CurrentView,
		timeoutCollectorsFactory,
	)

	timeoutAggregator, err := timeoutaggregator.NewTimeoutAggregator(
		log,
		metricsCollector,
		metricsCollector,
		metricsCollector,
		livenessData.CurrentView,
		timeoutCollectors,
	)
	require.NoError(t, err)

	hotstuffModules := &consensus.HotstuffModules{
		Forks:                       forks,
		Validator:                   validator,
		Notifier:                    hotstuffDistributor,
		Committee:                   committee,
		Signer:                      signer,
		Persist:                     persist,
		VoteCollectorDistributor:    voteAggregationDistributor.VoteCollectorDistributor,
		TimeoutCollectorDistributor: timeoutAggregationDistributor.TimeoutCollectorDistributor,
		VoteAggregator:              voteAggregator,
		TimeoutAggregator:           timeoutAggregator,
	}

	// initialize hotstuff
	hot, err := consensus.NewParticipant(
		log,
		metricsCollector,
		metricsCollector,
		build,
		rootHeader,
		[]*flow.ProposalHeader{},
		hotstuffModules,
		consensus.WithMinTimeout(hotstuffTimeout),
		func(cfg *consensus.ParticipantConfig) {
			cfg.MaxTimeoutObjectRebroadcastInterval = hotstuffTimeout
		},
	)
	require.NoError(t, err)

	// initialize the compliance engine
	compCore, err := compliance.NewCore(
		log,
		metricsCollector,
		metricsCollector,
		metricsCollector,
		metricsCollector,
		hotstuffDistributor,
		tracer,
		headersDB,
		payloadsDB,
		fullState,
		cache,
		syncCore,
		validator,
		hot,
		voteAggregator,
		timeoutAggregator,
		modulecompliance.DefaultConfig(),
	)
	require.NoError(t, err)

	comp, err := compliance.NewEngine(log, me, compCore)
	require.NoError(t, err)

	identities, err := state.Final().Identities(filter.And(
		filter.HasRole[flow.Identity](flow.RoleConsensus),
		filter.Not(filter.HasNodeID[flow.Identity](me.NodeID())),
	))
	require.NoError(t, err)
	idProvider := id.NewFixedIdentifierProvider(identities.NodeIDs())

	spamConfig, err := synceng.NewSpamDetectionConfig()
	require.NoError(t, err, "could not initialize spam detection config")

	// initialize the synchronization engine
	sync, err := synceng.New(
		log,
		metricsCollector,
		net,
		me,
		state,
		blocksDB,
		comp,
		syncCore,
		idProvider,
		spamConfig,
		func(cfg *synceng.Config) {
			// use a small pool and scan interval for sync engine
			cfg.ScanInterval = 500 * time.Millisecond
			cfg.PollInterval = time.Second
		},
	)
	require.NoError(t, err)

	messageHub, err := message_hub.NewMessageHub(
		log,
		metricsCollector,
		net,
		me,
		comp,
		hot,
		voteAggregator,
		timeoutAggregator,
		state,
		payloadsDB,
	)
	require.NoError(t, err)

	hotstuffDistributor.AddConsumer(messageHub)

	node.dbCloser = db
	node.compliance = comp
	node.sync = sync
	node.state = fullState
	node.hot = hot
	node.committee = committee
	node.voteAggregator = hotstuffModules.VoteAggregator
	node.timeoutAggregator = hotstuffModules.TimeoutAggregator
	node.messageHub = messageHub
	node.headers = headersDB
	node.net = net
	node.log = log

	return node
}

func cleanupNodes(nodes []*Node) {
	for _, n := range nodes {
		_ = n.dbCloser.Close()
		_ = os.RemoveAll(n.dbDir)
	}
}
