package dkg

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dkgmodel "github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/model/flow"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/state/protocol/events/gadgets"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

// TestEpochSetup ensures that, upon receiving an EpochSetup event, the engine
// correclty creates a new DKGController and registers phase transitions based
// on the views specified in the event, as well as regular calls to the DKG
// smart-contract.
//
// The EpochSetup event is received at view 100. The phase transitions are at
// views 150, 200, and 250. In between phase transitions, the controller calls
// the DKG smart-contract every 10 views.
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
func TestEpochSetup(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	currentCounter := rand.Uint64()
	nextCounter := currentCounter + 1
	committee := unittest.IdentityListFixture(10)
	myIndex := 5
	me := new(module.Local)
	me.On("NodeID").Return(committee[myIndex].NodeID)

	// create a block for each view of interest
	blocks := make(map[uint64]*flow.Header)
	var view uint64
	for view = 100; view <= 250; view += DefaultPollStep {
		blocks[view] = &flow.Header{View: view}
	}
	firstBlock := blocks[100]

	// expectedPrivKey is the expected private share produced by the dkg run. We
	// will mock the controller to return this value, and we will check it
	// against the value that gets inserted in the DB at the end.
	expectedPrivKey, _ := unittest.NetworkingKey()

	// insert epoch setup in mock state
	epochSetup := flow.EpochSetup{
		Counter:            nextCounter,
		DKGPhase1FinalView: 150,
		DKGPhase2FinalView: 200,
		DKGPhase3FinalView: 250,
		FinalView:          300,
		Participants:       committee,
		RandomSource:       []byte("random bytes"),
	}
	epoch := new(protocol.Epoch)
	epoch.On("Counter").Return(epochSetup.Counter, nil)
	epoch.On("InitialIdentities").Return(epochSetup.Participants, nil)
	epoch.On("DKGPhase1FinalView").Return(epochSetup.DKGPhase1FinalView, nil)
	epoch.On("DKGPhase2FinalView").Return(epochSetup.DKGPhase2FinalView, nil)
	epoch.On("DKGPhase3FinalView").Return(epochSetup.DKGPhase3FinalView, nil)
	epoch.On("Seed", mock.Anything, mock.Anything, mock.Anything).Return(epochSetup.RandomSource, nil)
	epochQuery := mocks.NewEpochQuery(t, currentCounter)
	epochQuery.Add(epoch)
	snapshot := new(protocol.Snapshot)
	snapshot.On("Epochs").Return(epochQuery)
	state := new(protocol.State)
	state.On("AtBlockID", firstBlock.ID()).Return(snapshot)

	// ensure that an attempt is made to insert the expected dkg private share
	// for the next epoch.
	keyStorage := new(storage.DKGKeys)
	keyStorage.On("InsertMyDKGPrivateInfo", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			epochCounter := args.Get(0).(uint64)
			require.Equal(t, nextCounter, epochCounter)
			dkgPriv := args.Get(1).(*dkgmodel.DKGParticipantPriv)
			require.Equal(t, me.NodeID(), dkgPriv.NodeID)
			require.Equal(t, expectedPrivKey, dkgPriv.RandomBeaconPrivKey.PrivateKey)
			require.Equal(t, myIndex, dkgPriv.GroupIndex)
		}).
		Return(nil).
		Once()

	// we will ensure that the controller state transitions get called
	// appropriately
	controller := new(module.DKGController)
	controller.On("Run").Return(nil).Once()
	controller.On("EndPhase1").Return(nil).Once()
	controller.On("EndPhase2").Return(nil).Once()
	controller.On("End").Return(nil).Once()
	controller.On("Poll", mock.Anything).Return(nil).Times(15)
	controller.On("GetArtifacts").Return(expectedPrivKey, nil, nil).Once()
	controller.On("SubmitResult").Return(nil).Once()

	factory := new(module.DKGControllerFactory)
	factory.On("Create",
		fmt.Sprintf("dkg-%d", epochSetup.Counter),
		committee.NodeIDs(),
		myIndex,
		epochSetup.RandomSource).Return(controller, nil)

	viewEvents := gadgets.NewViews()
	engine := NewReactorEngine(
		zerolog.New(ioutil.Discard),
		me,
		state,
		keyStorage,
		factory,
		viewEvents,
	)

	engine.EpochSetupPhaseStarted(epochSetup.Counter, firstBlock)

	for view = 100; view <= 250; view += DefaultPollStep {
		viewEvents.BlockFinalized(blocks[view])
	}

	// check that the appropriate callbacks were registered
	time.Sleep(50 * time.Millisecond)
	controller.AssertExpectations(t)
}
