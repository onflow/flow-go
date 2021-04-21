package dkg

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	dkgeng "github.com/onflow/flow-go/engine/consensus/dkg"
	"github.com/onflow/flow-go/engine/testutil"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/dkg"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/network/stub"
	"github.com/onflow/flow-go/state/protocol/events/gadgets"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

// create a set of nodes that share the same hub for networking, the same
// whiteboard for exchanging DKG broadcast messages, and a mocked state
// containing the expected next EpochSetup event
func createNodes(
	t *testing.T,
	hub *stub.Hub,
	chainID flow.ChainID,
	whiteboard *whiteboard,
	conIdentities flow.IdentityList,
	epochSetup flow.EpochSetup,
	firstBlockID flow.Identifier) ([]*node, flow.IdentityList) {

	// We need to initialise the nodes with a list of identities that contain
	// all roles, otherwise there would be an error initialising the first epoch
	identities := unittest.CompleteIdentitySet(conIdentities...)

	nodes := []*node{}
	for _, id := range conIdentities {
		nodes = append(nodes, createNode(t, id, identities, hub, chainID, whiteboard, epochSetup, firstBlockID))
	}

	return nodes, conIdentities
}

// createNode instantiates a node with a network hub, a whiteboard reference,
// and a pre-set EpochSetup that will be used to trigger the next DKG run.
func createNode(
	t *testing.T,
	id *flow.Identity,
	ids []*flow.Identity,
	hub *stub.Hub,
	chainID flow.ChainID,
	whiteboard *whiteboard,
	epochSetup flow.EpochSetup,
	firstBlock flow.Identifier) *node {

	core := testutil.GenericNode(t, hub, id, ids, chainID)
	core.Log = zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)

	// the viewsObserver is used by the reactor engine to subscribe to when
	// blocks are finalized that are in a new view
	viewsObserver := gadgets.NewViews()
	core.ProtocolEvents.AddConsumer(viewsObserver)

	// keyKeys is used to store the private key resulting from the node's
	// participation in the DKG run
	dkgKeys := badger.NewDKGKeys(core.Metrics, core.DB)

	// configure the state snapthost at firstBlock to return the desired
	// EpochSetup
	epoch := new(protocolmock.Epoch)
	epoch.On("Counter").Return(epochSetup.Counter, nil)
	epoch.On("InitialIdentities").Return(epochSetup.Participants, nil)
	epoch.On("DKGPhase1FinalView").Return(epochSetup.DKGPhase1FinalView, nil)
	epoch.On("DKGPhase2FinalView").Return(epochSetup.DKGPhase2FinalView, nil)
	epoch.On("DKGPhase3FinalView").Return(epochSetup.DKGPhase3FinalView, nil)
	epoch.On("Seed", mock.Anything, mock.Anything, mock.Anything).Return(epochSetup.RandomSource, nil)
	epochQuery := mocks.NewEpochQuery(t, epochSetup.Counter-1)
	epochQuery.Add(epoch)
	snapshot := new(protocolmock.Snapshot)
	snapshot.On("Epochs").Return(epochQuery)
	state := new(protocolmock.MutableState)
	state.On("AtBlockID", firstBlock).Return(snapshot)
	core.State = state

	// brokerTunnel is used to communicate between the messaging engine and the
	// DKG broker/controller
	brokerTunnel := dkg.NewBrokerTunnel()

	// messagingEngine is a network engine that is used by nodes to exchange
	// private DKG messages
	messagingEngine, err := dkgeng.NewMessagingEngine(
		core.Log,
		core.Net,
		core.Me,
		brokerTunnel,
	)
	require.NoError(t, err)

	// the reactor engine reacts to new views being finalized and drives the
	// DKG protocol
	reactorEngine := dkgeng.NewReactorEngine(
		core.Log,
		core.Me,
		core.State,
		dkgKeys,
		dkg.NewControllerFactory(
			core.Log,
			core.Me,
			NewWhiteboardClient(id.NodeID, whiteboard),
			brokerTunnel,
		),
		viewsObserver,
	)

	// reactorEngine consumes the EpochSetupPhaseStarted event
	core.ProtocolEvents.AddConsumer(reactorEngine)

	node := node{
		GenericNode:     core,
		keyStorage:      dkgKeys,
		messagingEngine: messagingEngine,
		reactorEngine:   reactorEngine,
	}

	return &node
}

func TestWithWhiteboard(t *testing.T) {

	// hub is an in-memory test network that enables nodes to communicate using
	// the DKG messaging engine
	hub := stub.NewNetworkHub()

	// whiteboard is a shared object where DKG nodes can publish/read broadcast
	// messages, as well as publish end results, using a special
	// DKGContractClient.
	// TODO: replace with a real smart-contract and emulator
	whiteboard := newWhiteboard()

	chainID := flow.Testnet

	// we run the DKG protocol with N consensus nodes
	N := 10
	conIdentities := unittest.IdentityListFixture(N, unittest.WithRole(flow.RoleConsensus))

	// The EpochSetup event is received at view 100. The phase transitions are
	// at views 150, 200, and 250. In between phase transitions, the controller
	// calls the DKG smart-contract every 10 views.
	//
	// VIEWS
	// setup      : 100
	// polling    : 110 120 130 140 150
	// Phase1Final: 150
	// polling    : 160 170 180 190 200
	// Phase2Final: 200
	// polling    : 210 220 230 240 250
	// Phase3Final: 250
	// final      : 300

	// create and record relevant blocks
	blocks := make(map[uint64]*flow.Header)
	var view uint64
	for view = 100; view <= 250; view += dkgeng.DefaultPollStep {
		blocks[view] = &flow.Header{View: view}
	}
	firstBlock := blocks[100]

	// create the EpochSetup that will trigger the next DKG run with all the
	// desired parameters
	epochSetup := flow.EpochSetup{
		Counter:            999,
		DKGPhase1FinalView: 150,
		DKGPhase2FinalView: 200,
		DKGPhase3FinalView: 250,
		FinalView:          300,
		Participants:       conIdentities,
		RandomSource:       []byte("random bytes for seed"),
	}

	nodes, _ := createNodes(
		t,
		hub,
		chainID,
		whiteboard,
		conIdentities,
		epochSetup,
		firstBlock.ID())

	for _, n := range nodes {
		n.Ready()
	}

	// trigger the EpochSetupPhaseStarted event for all nodes, effectively
	// starting the next DKG run
	for _, n := range nodes {
		// n.reactorEngine.EpochSetupPhaseStarted(epochSetup.Counter, firstBlock)
		n.ProtocolEvents.EpochSetupPhaseStarted(epochSetup.Counter, firstBlock)
	}

	// trigger the BlockFinalized events for each view of interest, effectively
	// causing the DKG state machine to progress
	for view = 100; view <= 250; view += dkgeng.DefaultPollStep {
		time.Sleep(300 * time.Millisecond)
		hub.DeliverAll()
		for _, n := range nodes {
			n.ProtocolEvents.BlockFinalized(blocks[view])
		}
	}

	for _, n := range nodes {
		n.Done()
	}

	t.Logf("there are %d result(s)", len(whiteboard.results))
	assert.Equal(t, 1, len(whiteboard.results))

	for _, result := range whiteboard.results {
		signers := whiteboard.resultSubmitters[flow.MakeID(result)]
		t.Logf("result %s has %d proposers", flow.MakeID(result), len(signers))
		assert.Equal(t, N, len(signers))
	}

	// create and test a threshold signature with the keys computed by dkg
	sigData := []byte("message to be signed")
	signers := make([]*signature.ThresholdProvider, 0, N)
	signatures := []crypto.Signature{}
	indices := []uint{}
	for i, n := range nodes {
		priv, err := n.keyStorage.RetrieveMyDKGPrivateInfo(epochSetup.Counter)
		require.NoError(t, err)

		signer := signature.NewThresholdProvider("TAG", priv.RandomBeaconPrivKey.PrivateKey)
		signers = append(signers, signer)

		signature, err := signer.Sign(sigData)
		require.NoError(t, err)

		signatures = append(signatures, signature)
		indices = append(indices, uint(i))
	}

	groupSignature, err := signature.CombineThresholdShares(uint(len(nodes)), signatures, indices)
	require.NoError(t, err)

	for i, n := range nodes {
		result := whiteboard.resultBySubmitter[n.Me.NodeID()]
		groupPk := result.groupKey
		t.Logf("group public key of node %d is %s", i, groupPk)
		ok, err := signers[i].Verify(sigData, groupSignature, groupPk)
		require.NoError(t, err)
		assert.True(t, ok, fmt.Sprintf("node %d fails to verify threshold signature", i))
	}
}
