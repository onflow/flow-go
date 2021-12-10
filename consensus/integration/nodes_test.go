package integration_test

import (
	bootstrapDKG "github.com/onflow/flow-go/cmd/bootstrap/dkg"
	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow/order"
	"os"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/gammazero/workerpool"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/consensus/hotstuff/persister"
	hsig "github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/consensus/hotstuff/voteaggregator"
	"github.com/onflow/flow-go/consensus/hotstuff/votecollector"
	synceng "github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/engine/consensus/compliance"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/buffer"
	builder "github.com/onflow/flow-go/module/builder/consensus"
	finalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/local"
	consensusMempools "github.com/onflow/flow-go/module/mempool/consensus"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	synccore "github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/state/protocol"
	bprotocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/blocktimer"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/state/protocol/util"
	storage "github.com/onflow/flow-go/storage/badger"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

const hotstuffTimeout = 100 * time.Millisecond

type ConsensusParticipants struct {
	lookup map[flow.Identifier]run.Participant
}

func NewConsensusParticipants(data *run.ParticipantData) *ConsensusParticipants {
	lookup := make(map[flow.Identifier]run.Participant)
	for _, pariticpant := range data.Participants {
		lookup[pariticpant.NodeID] = pariticpant
	}
	return &ConsensusParticipants{
		lookup: lookup,
	}
}

func (p *ConsensusParticipants) Lookup(nodeID flow.Identifier) *run.Participant {
	participant, ok := p.lookup[nodeID]
	if ok {
		return &participant
	}
	return nil
}

func (p *ConsensusParticipants) AddParticipants(participants ...run.Participant) {
	for _, participant := range participants {
		p.lookup[participant.NodeID] = participant
	}
}

type Node struct {
	db         *badger.DB
	dbDir      string
	index      int
	log        zerolog.Logger
	id         *flow.Identity
	compliance *compliance.Engine
	sync       *synceng.Engine
	hot        module.HotStuff
	aggregator hotstuff.VoteAggregator
	state      *bprotocol.MutableState
	headers    *storage.Headers
	net        *Network
}

func (n *Node) Shutdown() {
	<-n.sync.Done()
	<-n.compliance.Done()
}

// n - the total number of nodes to be created
// finalizedCount - the number of finalized blocks before stopping the tests
// tolerate - the number of node to tolerate that don't need to reach the finalization count
// 						before stopping the tests
func createNodes(t *testing.T, participants *ConsensusParticipants, rootSnapshot protocol.Snapshot, stopper *Stopper) ([]*Node, *Hub) {
	consensus, err := rootSnapshot.Identities(filter.HasRole(flow.RoleConsensus))
	require.NoError(t, err)

	hub := NewNetworkHub()
	nodes := make([]*Node, 0, len(consensus))
	for i, identity := range consensus {
		consensusParticipant := participants.Lookup(identity.NodeID)
		require.NotNil(t, consensusParticipant)
		node := createNode(t, consensusParticipant, i, identity, rootSnapshot, hub, stopper)
		nodes = append(nodes, node)
	}

	return nodes, hub
}

func createRootQC(t *testing.T, root *flow.Block, participantData *run.ParticipantData) *flow.QuorumCertificate {
	consensusCluster := participantData.Identities()
	votes, err := run.GenerateRootBlockVotes(root, participantData)
	require.NoError(t, err)
	qc, err := run.GenerateRootQC(root, votes, participantData, consensusCluster)
	require.NoError(t, err)
	return qc
}

func createRootBlockData(participantData *run.ParticipantData) (*flow.Block, *flow.ExecutionResult, *flow.Seal) {
	root := unittest.GenesisFixture()
	consensusParticipants := participantData.Identities()

	// add other roles to create a complete identity list
	participants := unittest.CompleteIdentitySet(consensusParticipants...)
	participants.Sort(order.ByNodeIDAsc)

	dkgParticipantsKeys := make([]crypto.PublicKey, 0, len(consensusParticipants))
	for _, participant := range participants.Filter(filter.HasRole(flow.RoleConsensus)) {
		dkgParticipantsKeys = append(dkgParticipantsKeys, participantData.Lookup[participant.NodeID].KeyShare)
	}

	counter := uint64(1)
	setup := unittest.EpochSetupFixture(
		unittest.WithParticipants(participants),
		unittest.SetupWithCounter(counter),
		unittest.WithFirstView(root.Header.View),
		unittest.WithFinalView(root.Header.View+1000),
	)
	commit := unittest.EpochCommitFixture(
		unittest.CommitWithCounter(counter),
		unittest.WithClusterQCsFromAssignments(setup.Assignments),
		func(commit *flow.EpochCommit) {
			commit.DKGGroupKey = participantData.GroupKey
			commit.DKGParticipantKeys = dkgParticipantsKeys
		},
	)

	result := unittest.BootstrapExecutionResultFixture(root, unittest.GenesisStateCommitment)
	result.ServiceEvents = []flow.ServiceEvent{setup.ServiceEvent(), commit.ServiceEvent()}

	seal := unittest.Seal.Fixture(unittest.Seal.WithResult(result))

	return root, result, seal
}

func createPrivateNodeIdentities(n int) []bootstrap.NodeInfo {
	consensus := unittest.IdentityListFixture(n, unittest.WithRole(flow.RoleConsensus)).Sort(order.ByNodeIDAsc)
	infos := make([]bootstrap.NodeInfo, 0, n)
	for _, node := range consensus {
		networkPrivKey := unittest.NetworkingPrivKeyFixture()
		stakingPrivKey := unittest.StakingPrivKeyFixture()
		nodeInfo := bootstrap.NewPrivateNodeInfo(
			node.NodeID,
			node.Role,
			node.Address,
			node.Stake,
			networkPrivKey,
			stakingPrivKey,
		)
		infos = append(infos, nodeInfo)
	}
	return infos
}

func createConsensusIdentities(t *testing.T, n int) *run.ParticipantData {
	// create n consensus node participants
	consensus := createPrivateNodeIdentities(n)
	return completeConsensusIdentities(t, consensus)
}

// completeConsensusIdentities runs KG process and fills nodeInfos with missing random beacon keys
func completeConsensusIdentities(t *testing.T, nodeInfos []bootstrap.NodeInfo) *run.ParticipantData {
	dkgData, err := bootstrapDKG.RunFastKG(len(nodeInfos), unittest.RandomBytes(48))
	require.NoError(t, err)

	participantData := &run.ParticipantData{
		Participants: make([]run.Participant, 0, len(nodeInfos)),
		Lookup:       make(map[flow.Identifier]flow.DKGParticipant),
		GroupKey:     dkgData.PubGroupKey,
	}
	for index, node := range nodeInfos {
		participant := run.Participant{
			NodeInfo:            node,
			RandomBeaconPrivKey: dkgData.PrivKeyShares[index],
		}
		participantData.Participants = append(participantData.Participants, participant)
		participantData.Lookup[node.NodeID] = flow.DKGParticipant{
			Index:    uint(index),
			KeyShare: dkgData.PubKeyShares[index],
		}
	}

	return participantData
}

func createRootSnapshot(t *testing.T, participantData *run.ParticipantData) *inmem.Snapshot {
	root, result, seal := createRootBlockData(participantData)
	rootQC := createRootQC(t, root, participantData)

	rootSnapshot, err := inmem.SnapshotFromBootstrapState(root, result, seal, rootQC)
	require.NoError(t, err)
	return rootSnapshot
}

func createNode(
	t *testing.T,
	participant *run.Participant,
	index int,
	identity *flow.Identity,
	rootSnapshot protocol.Snapshot,
	hub *Hub,
	stopper *Stopper,
) *Node {

	db, dbDir := unittest.TempBadgerDB(t)
	metrics := metrics.NewNoopCollector()
	tracer := trace.NewNoopTracer()

	headersDB := storage.NewHeaders(metrics, db)
	guaranteesDB := storage.NewGuarantees(metrics, db, storage.DefaultCacheSize)
	sealsDB := storage.NewSeals(metrics, db)
	indexDB := storage.NewIndex(metrics, db)
	resultsDB := storage.NewExecutionResults(metrics, db)
	receiptsDB := storage.NewExecutionReceipts(metrics, db, resultsDB, storage.DefaultCacheSize)
	payloadsDB := storage.NewPayloads(db, indexDB, guaranteesDB, sealsDB, receiptsDB, resultsDB)
	blocksDB := storage.NewBlocks(db, headersDB, payloadsDB)
	setupsDB := storage.NewEpochSetups(metrics, db)
	commitsDB := storage.NewEpochCommits(metrics, db)
	statusesDB := storage.NewEpochStatuses(metrics, db)
	consumer := events.NewDistributor()

	state, err := bprotocol.Bootstrap(metrics, db, headersDB, sealsDB, resultsDB, blocksDB, setupsDB, commitsDB, statusesDB, rootSnapshot)
	require.NoError(t, err)

	blockTimer, err := blocktimer.NewBlockTimer(1*time.Millisecond, 90*time.Second)
	require.NoError(t, err)

	fullState, err := bprotocol.NewFullConsensusState(state, indexDB, payloadsDB, tracer, consumer,
		blockTimer, util.MockReceiptValidator(), util.MockSealValidator(sealsDB))
	require.NoError(t, err)

	localID := identity.ID()

	node := &Node{
		db:    db,
		dbDir: dbDir,
		index: index,
		id:    identity,
	}

	// log with node index an ID
	log := unittest.Logger().With().
		Int("index", index).
		Hex("node_id", localID[:]).
		Logger()

	stopConsumer := stopper.AddNode(node)

	counterConsumer := &CounterConsumer{
		finalized: func(total uint) {
			stopper.onFinalizedTotal(node.id.ID(), total)
		},
	}

	// log with node index
	logConsumer := notifications.NewLogConsumer(log)
	notifier := pubsub.NewDistributor()
	notifier.AddConsumer(stopConsumer)
	notifier.AddConsumer(counterConsumer)
	notifier.AddConsumer(logConsumer)

	cleaner := &storagemock.Cleaner{}
	cleaner.On("RunGC")

	require.Equal(t, participant.NodeID, localID)
	privateKeys, err := participant.PrivateKeys()
	require.NoError(t, err)

	// make local
	me, err := local.New(identity, privateKeys.StakingKey)
	require.NoError(t, err)

	// add a network for this node to the hub
	net := hub.AddNetwork(localID, node)

	guaranteeLimit, sealLimit := uint(1000), uint(1000)
	guarantees, err := stdmap.NewGuarantees(guaranteeLimit)
	require.NoError(t, err)

	receipts := consensusMempools.NewExecutionTree()

	seals := stdmap.NewIncorporatedResultSeals(sealLimit)

	// initialize the block builder
	build, err := builder.NewBuilder(metrics, db, fullState, headersDB, sealsDB, indexDB, blocksDB, resultsDB, receiptsDB,
		guarantees, consensusMempools.NewIncorporatedResultSeals(seals, receiptsDB), receipts, tracer)
	require.NoError(t, err)

	// initialize the pending blocks cache
	cache := buffer.NewPendingBlocks()

	rootHeader, err := rootSnapshot.Head()
	require.NoError(t, err)

	rootQC, err := rootSnapshot.QuorumCertificate()
	require.NoError(t, err)

	// selector := filter.HasRole(flow.RoleConsensus)
	committee, err := committees.NewConsensusCommittee(state, localID)
	require.NoError(t, err)

	// initialize the block finalizer
	final := finalizer.NewFinalizer(db, headersDB, fullState, trace.NewNoopTracer())

	prov := &mocknetwork.Engine{}
	prov.On("SubmitLocal", mock.Anything).Return(nil)

	syncCore, err := synccore.New(log, synccore.DefaultConfig())
	require.NoError(t, err)

	qcDistributor := pubsub.NewQCCreatedDistributor()

	forks, err := consensus.NewForks(rootHeader, headersDB, final, notifier, rootHeader, rootQC)
	require.NoError(t, err)

	validator := consensus.NewValidator(metrics, committee, forks)
	require.NoError(t, err)

	beaconSignerStore := hsig.NewStaticRandomBeaconSignerStore(participant.RandomBeaconPrivKey)
	signer := verification.NewCombinedSigner(me, beaconSignerStore)

	persist := persister.New(db, rootHeader.ChainID)

	started, err := persist.GetStarted()
	require.NoError(t, err)

	voteProcessorFactory := votecollector.NewCombinedVoteProcessorFactory(committee, qcDistributor.OnQcConstructedFromVotes)

	createCollectorFactoryMethod := votecollector.NewStateMachineFactory(log, notifier, voteProcessorFactory.Create)
	voteCollectors := voteaggregator.NewVoteCollectors(started, workerpool.New(2), createCollectorFactoryMethod)

	aggregator, err := voteaggregator.NewVoteAggregator(log, notifier, started, voteCollectors)
	require.NoError(t, err)

	hotstuffModules := &consensus.HotstuffModules{
		Forks:                forks,
		Validator:            validator,
		Notifier:             notifier,
		Committee:            committee,
		Signer:               signer,
		Persist:              persist,
		QCCreatedDistributor: qcDistributor,
		Aggregator:           aggregator,
	}

	// initialize the compliance engine
	compCore, err := compliance.NewCore(
		log,
		metrics,
		tracer,
		metrics,
		metrics,
		cleaner,
		headersDB,
		payloadsDB,
		fullState,
		cache,
		syncCore,
		aggregator,
	)
	require.NoError(t, err)

	comp, err := compliance.NewEngine(log, net, me, prov, compCore)
	require.NoError(t, err)

	finalizedHeader, err := synceng.NewFinalizedHeaderCache(log, state, pubsub.NewFinalizationDistributor())
	require.NoError(t, err)

	identities, err := state.Final().Identities(filter.And(
		filter.HasRole(flow.RoleConsensus),
		filter.Not(filter.HasNodeID(me.NodeID())),
	))
	require.NoError(t, err)
	idProvider := id.NewFixedIdentifierProvider(identities.NodeIDs())

	// initialize the synchronization engine
	sync, err := synceng.New(
		log,
		metrics,
		net,
		me,
		blocksDB,
		comp,
		syncCore,
		finalizedHeader,
		idProvider,
	)
	require.NoError(t, err)

	// initialize the block finalizer
	hot, err := consensus.NewParticipant(
		log,
		metrics,
		build,
		comp,
		rootHeader,
		[]*flow.Header{},
		hotstuffModules,
		consensus.WithInitialTimeout(hotstuffTimeout),
		consensus.WithMinTimeout(hotstuffTimeout),
	)

	require.NoError(t, err)

	comp = comp.WithConsensus(hot)

	node.compliance = comp
	node.sync = sync
	node.state = fullState
	node.hot = hot
	node.aggregator = hotstuffModules.Aggregator
	node.headers = headersDB
	node.net = net
	node.log = log

	return node
}

func cleanupNodes(nodes []*Node) {
	for _, n := range nodes {
		_ = n.db.Close()
		_ = os.RemoveAll(n.dbDir)
	}
}
