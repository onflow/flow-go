/*

Package dkg implements a controller that manages the lifecycle of a Joint
Feldman DKG node.

The state-machine can be represented as follows:

+-------+  /Run() +---------+  /EndPhase0() +---------+  /EndPhase1() +---------+  /End()   +----------+
| Init  | ----->  | Phase 0 | ------------> | Phase 1 | ------------> | Phase 2 | --------> | Shutdown |
+-------+         +---------+               +---------+               +---------+           +----------+
   |                   |                         |                         |                      ^
   v___________________v_________________________v_________________________v______________________|
                                            /Shutdown()

The controller is always in one of 5 states:
	- Init: Default state before the instance is started
	- Phase 0: 1st phase of the JF DKG protocol while it's running
	- Phase 1: 2nd phase of the JF DKG protocol while it's running
	- Phase 2: 3rd phase of the JF DKG protocol while it's running
	- Shutdown: When the controller is stopped

The controller exposes the following functions to trigger transitions:

Run(): Triggers transition from Init to Phase0. Starts the DKG protocol instance
       and background communication routines.

EndPhase0(): Triggers transition from Phase 0 to Phase 1.

EndPhase1(): Triggers transition from Phase 1 to Phase 2.

End(): Ends the DKG protocol and returns the artifacts. Triggers transition to
	   Shutdown.

Shutdown(): Can be called from any state to stop the DKG instance.

*/
package dkg

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/crypto"
)

// Controller controls the execution of a Joint Feldman DKG instance.
type Controller struct {
	// The embedded state Manager is used to manage the controller's underlying
	// state.
	Manager

	// DKGState is the object that actually executes the protocol steps.
	dkg crypto.DKGState

	// seed is required by DKGState
	seed []byte

	// msgCh is the channel through which the controller receives private and
	// broadcast messages. It is assumed that the messages are valid. All
	// validity checks should be performed upstream.
	msgCh chan DKGMessage

	// Channels used internally to trigger state transitions
	h0Ch       chan struct{}
	h1Ch       chan struct{}
	endCh      chan struct{}
	shutdownCh chan struct{}

	log zerolog.Logger
}

// NewController instantiates a new Joint Feldman DKG controller.
func NewController(
	dkg crypto.DKGState,
	seed []byte,
	msgCh chan DKGMessage,
	log zerolog.Logger) *Controller {

	return &Controller{
		log:        log,
		dkg:        dkg,
		seed:       seed,
		msgCh:      msgCh,
		h0Ch:       make(chan struct{}),
		h1Ch:       make(chan struct{}),
		endCh:      make(chan struct{}),
		shutdownCh: make(chan struct{}),
	}
}

/*******************************************************************************
CONTROLS
*******************************************************************************/

// Run starts the DKG controller. It is a blocking call that blocks until the
// controller is shutdown or until an error is encountered in one of the
// protocol phases.
func (c *Controller) Run() error {

	// Start DKG and transition to phase 0
	err := c.start()
	if err != nil {
		return err
	}

	// Start a background routine to listen for incoming private and broadcast
	// messages from other nodes
	go c.doBackgroundWork()

	// Execute DKG State Machine
	for {
		state := c.GetState()
		c.log.Debug().Msgf("DKG: %s", c.state)

		switch state {
		case Phase0:
			err := c.phase0()
			if err != nil {
				return err
			}
		case Phase1:
			err := c.phase1()
			if err != nil {
				return err
			}
		case Phase2:
			err := c.phase2()
			if err != nil {
				return err
			}
		case Shutdown:
			return nil
		}
	}
}

// EndPhase0 notifies the controller to end phase 0, and start phase 1
func (c *Controller) EndPhase0() error {
	state := c.GetState()
	if state != Phase0 {
		return NewInvalidStateTransitionError(state, Phase1)
	}

	close(c.h0Ch)

	c.SetState(Phase1)

	return nil
}

// EndPhase1 notifies the controller to end phase 1, and start phase 2
func (c *Controller) EndPhase1() error {
	state := c.GetState()
	if state != Phase1 {
		return NewInvalidStateTransitionError(state, Phase2)
	}

	close(c.h1Ch)

	c.SetState(Phase2)

	return nil
}

// End terminates the DKG state machine and returns the artifacts
func (c *Controller) End() (crypto.PrivateKey, crypto.PublicKey, []crypto.PublicKey, error) {
	state := c.GetState()
	if state != Phase2 {
		return nil, nil, nil, NewInvalidStateTransitionError(state, Shutdown)
	}

	close(c.endCh)

	c.SetState(Shutdown)

	c.log.Debug().Msg("DKG engine end")

	// return the products of the DKG protocol
	return c.dkg.End()
}

// Shutdown stops the controller regardless of the current state.
func (c *Controller) Shutdown() {
	close(c.shutdownCh)
}

/*******************************************************************************
WORKERS
*******************************************************************************/

func (c *Controller) doBackgroundWork() {
	for {
		select {
		case msg := <-c.msgCh:
			err := c.dkg.HandleMsg(msg.Orig, msg.Data)
			if err != nil {
				c.log.Err(err).Msg("Error processing DKG message")
			}
		case <-c.shutdownCh:
			return
		}
	}
}

func (c *Controller) start() error {
	state := c.GetState()
	if state != Init {
		return fmt.Errorf("Cannot execute start routine in state %s", state)
	}

	err := c.dkg.Start(c.seed)
	if err != nil {
		return fmt.Errorf("Error starting DKG: %w", err)
	}

	c.log.Debug().Msg("DKG engine started")

	c.SetState(Phase0)

	return nil
}

func (c *Controller) phase0() error {
	state := c.GetState()
	if state != Phase0 {
		return fmt.Errorf("Cannot execute phase0 routine in state %s", state)
	}

	c.log.Debug().Msg("Waiting for end of phase 0")

	for {
		select {
		case <-c.h0Ch:
			return nil
		case <-c.shutdownCh:
			c.SetState(Shutdown)
			return nil
		}
	}
}

func (c *Controller) phase1() error {
	state := c.GetState()
	if state != Phase1 {
		return fmt.Errorf("Cannot execute phase1 routine in state %s", state)
	}

	err := c.dkg.NextTimeout()
	if err != nil {
		return fmt.Errorf("Error calling NextTimeout: %w", err)
	}

	c.log.Debug().Msg("Waiting for end of phase 1")

	for {
		select {
		case <-c.h1Ch:
			return nil
		case <-c.shutdownCh:
			c.SetState(Shutdown)
			return nil
		}
	}
}

func (c *Controller) phase2() error {
	state := c.GetState()
	if state != Phase2 {
		return fmt.Errorf("Cannot execute phase2 routine in state %s", state)
	}

	err := c.dkg.NextTimeout()
	if err != nil {
		return fmt.Errorf("Error calling NextTimeout: %w", err)
	}

	c.log.Debug().Msg("Waiting for end of phase 2")

	for {
		select {
		case <-c.endCh:
			return nil
		case <-c.shutdownCh:
			c.SetState(Shutdown)
			return nil
		}
	}
}
