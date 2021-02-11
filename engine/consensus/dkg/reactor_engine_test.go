package dkg

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/model/flow"
	module "github.com/onflow/flow-go/module/mock"
	events "github.com/onflow/flow-go/state/protocol/events"
	mockEvents "github.com/onflow/flow-go/state/protocol/events/mock"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
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

	epochSetup := flow.EpochSetup{
		Counter:            rand.Uint64(),
		DKGPhase1FinalView: 150,
		DKGPhase2FinalView: 200,
		DKGPhase3FinalView: 250,
		FinalView:          300,
		Participants:       committee,
		RandomSource:       []byte("random bytes"),
	}

	setups := new(storage.EpochSetups)
	setups.On("ByID", epochSetup.ID()).Return(&epochSetup, nil)

	firstBlock := blocks[100]
	statuses := new(storage.EpochStatuses)
	statuses.On("ByBlockID", firstBlock.ID()).Return(
		&flow.EpochStatus{
			CurrentEpoch: flow.EventIDs{SetupID: epochSetup.ID()},
		},
		nil,
	)

	// we will ensure that the controller state transitions get called
	// appropriately
	controller := new(module.DKGController)
	controller.On("Run").Return(nil).Once()
	controller.On("EndPhase1").Return(nil).Once()
	controller.On("EndPhase2").Return(nil).Once()
	controller.On("End").Return(nil).Once()
	controller.On("Poll", mock.Anything).Return(nil).Times(15)

	factory := new(module.DKGControllerFactory)
	factory.On("Create",
		fmt.Sprintf("dkg-%d", epochSetup.Counter),
		committee.NodeIDs(),
		myIndex,
		epochSetup.RandomSource).Return(controller, nil)

	// record all the callback methods that get registered with OnView
	viewEvents := new(mockEvents.Views)
	transitionCallbacks := make(map[uint64]events.OnViewCallback)
	pollCallbacks := make(map[uint64]events.OnViewCallback)
	viewEvents.On("OnView", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			view := args.Get(0).(uint64)
			callback := args.Get(1).(events.OnViewCallback)
			_, recordedTransition := transitionCallbacks[view]
			// record the callback in the appropriate category. this is
			// necessary because views corresponding to phase transitions have
			// two callbacks, one for the actual phase transition and one for
			// calling the smart-contract.
			if (view == epochSetup.DKGPhase1FinalView ||
				view == epochSetup.DKGPhase2FinalView ||
				view == epochSetup.DKGPhase3FinalView) &&
				!recordedTransition {
				// if the view corresponds to a phase transition, and we haven't
				// already recorded one, record this callback as a phase
				// transition
				transitionCallbacks[view] = callback
			} else {
				pollCallbacks[view] = callback
			}
		}).
		Times(18) // a total of 18 calls to OnView must be made

	engine := NewReactorEngine(
		zerolog.New(ioutil.Discard),
		me,
		setups,
		statuses,
		factory,
		viewEvents,
	)

	engine.EpochSetupPhaseStarted(epochSetup.Counter, firstBlock)

	// check that enough calls to OnView were made
	viewEvents.AssertExpectations(t)

	// trigger callbacks as if new views were finalized
	for view, callback := range transitionCallbacks {
		header := blocks[view]
		callback(header)
	}
	for view, callback := range pollCallbacks {
		header := blocks[view]
		callback(header)
	}

	// check that the appropriate callbacks were registered
	time.Sleep(1 * time.Second)
	controller.AssertExpectations(t)
}
