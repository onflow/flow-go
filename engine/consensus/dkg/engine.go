package dkg

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	dkgmod "github.com/onflow/flow-go/module/dkg"
	"github.com/onflow/flow-go/state/protocol/events"
)

type Engine struct {
	events.Noop

	unit *engine.Unit
	log  zerolog.Logger
	me   module.Local

	controller        dkgmod.Controller
	controllerFactory dkgmod.ControllerFactory

	heightEvents events.Heights // allows subscribing to particular heights
}

func New(
	log zerolog.Logger,
	me module.Local,
	controllerFactory dkgmod.ControllerFactory,
	heightEvents events.Heights,
) *Engine {

	return &Engine{
		unit:              engine.NewUnit(),
		log:               log,
		me:                me,
		controllerFactory: controllerFactory,
		heightEvents:      heightEvents,
	}
}

// Ready implements the module ReadyDoneAware interface. It returns a channel
// that will close when the engine has successfully
// started.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done implements the module ReadyDoneAware interface. It returns a channel
// that will close when the engine has successfully stopped.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// EpochSetupPhaseStarted handles the EpochSetupPhaseStared protocol event.
//
// The parameters are:
// * counter: the epoch identifier
// * first  : the block that sealed the service event
func (e *Engine) EpochSetupPhaseStarted(counter uint64, first *flow.Header) {
	// 1) fetch corresponding EpochSetup event
	// 2) instantiate new controller
	// 3) reset processor engine
	// 4) set callbacks for phase transitions
	// 5) set callbacks for fetching broadcast messages

	controller, _ := e.controllerFactory.Create(
		fmt.Sprintf("dkg-%d", counter),
		[]byte{},
		10,
		1,
	)

	firstHeight := first.Height
	endPhase0Height := firstHeight + 100
	endPhase1Height := endPhase0Height + 100
	endHeight := endPhase1Height + 100

	e.heightEvents.OnHeight(endPhase0Height, func() {
		e.unit.Launch(func() {
			e.unit.Lock()
			defer e.unit.Unlock()

			log.Info().Msg("ending DKG phase 0...")

			err := e.controller.EndPhaseO()
			if err != nil {
				e.log.Error().Err(err).Msgf("failed to end phase 0")
				return
			}

			log.Info().Msg("ended phase 0 successfully")
		})
	})

	e.heightEvents.OnHeight(endPhase1Height, func() {
		e.unit.Launch(func() {
			e.unit.Lock()
			defer e.unit.Unlock()

			log.Info().Msg("ending DKG phase 1...")

			err := e.controller.EndPhase1()
			if err != nil {
				e.log.Error().Err(err).Msgf("failed to end phase 1")
				return
			}

			log.Info().Msg("ended phase 1 successfully")
		})
	})

	e.heightEvents.OnHeight(endHeight, func() {
		e.unit.Launch(func() {
			e.unit.Lock()
			defer e.unit.Unlock()

			log.Info().Msg("ending DKG...")

			err := e.controller.End()
			if err != nil {
				e.log.Error().Err(err).Msgf("failed to end DKG")
				return
			}

			log.Info().Msg("ended DKG successfully")
		})
	})

	e.unit.Launch(func() { controller.Run() })
}
