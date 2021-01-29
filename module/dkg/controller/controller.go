/*

Package controller implements a controller that manages the lifecycle of a Joint
Feldman DKG node.

The state-machine can be represented as follows:

+-------+  /Run() +---------+  /EndPhase0() +---------+  /EndPhase1() +---------+  /End()   +-----+     +----------+
| Init  | ----->  | Phase 0 | ------------> | Phase 1 | ------------> | Phase 2 | --------> | End | --> | Shutdown |
+-------+         +---------+               +---------+               +---------+           +-----+     +----------+
   |                   |                         |                         |                                 ^
   v___________________v_________________________v_________________________v_________________________________|
                                            /Shutdown()

The controller is always in one of 6 states:
	- Init: Default state before the instance is started
	- Phase 0: 1st phase of the JF DKG protocol while it's running
	- Phase 1: 2nd phase of the JF DKG protocol while it's running
	- Phase 2: 3rd phase of the JF DKG protocol while it's running
	- End: When the DKG protocol is finished
	- Shutdown: When the controller and all its routines are stopped

The controller exposes the following functions to trigger transitions:

Run(): Triggers transition from Init to Phase0. Starts the DKG protocol instance
       and background communication routines.

EndPhase0(): Triggers transition from Phase 0 to Phase 1.

EndPhase1(): Triggers transition from Phase 1 to Phase 2.

End(): Ends the DKG protocol and records the artifacts in controller. Triggers
       transition from Phase 2 to End.

Shutdown(): Can be called from any state to stop the DKG instance.

The End and Shutdown states differ in that the End state can only be arrived at
from Phase 2 and after successfully computing the DKG artifacts. Whereas the
Shutdown state can be reached from any other state.

*/
package controller

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/crypto"
	msg "github.com/onflow/flow-go/model/messages"
)

// Controller controls the execution of a Joint Feldman DKG instance.
type Controller struct {
	// The embedded state Manager is used to manage the controller's underlying
	// state.
	Manager

	// DKGState is the object that actually executes the protocol steps.
	dkg crypto.DKGState

	// dkgLock protects access to dkg
	dkgLock sync.Mutex

	// seed is required by DKGState
	seed []byte

	// msgCh is the channel through which the controller receives private and
	// broadcast messages. It is assumed that the messages are valid. All
	// validity checks should be performed upstream.
	msgCh chan msg.DKGMessage

	// Channels used internally to trigger state transitions
	h0Ch       chan struct{}
	h1Ch       chan struct{}
	endCh      chan struct{}
	shutdownCh chan struct{}

	// private fields that hold the DKG artifacts when the protocol runs to
	// completion
	privateShare   crypto.PrivateKey
	publicKeys     []crypto.PublicKey
	groupPublicKey crypto.PublicKey

	// artifactsLock protects access to artifacts
	artifactsLock sync.Mutex

	log zerolog.Logger
}

// NewController instantiates a new Joint Feldman DKG controller.
func NewController(
	dkg crypto.DKGState,
	seed []byte,
	msgCh chan msg.DKGMessage,
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
		case End:
			c.Shutdown()
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

	c.SetState(Phase1)
	close(c.h0Ch)

	return nil
}

// EndPhase1 notifies the controller to end phase 1, and start phase 2
func (c *Controller) EndPhase1() error {
	state := c.GetState()
	if state != Phase1 {
		return NewInvalidStateTransitionError(state, Phase2)
	}

	c.SetState(Phase2)
	close(c.h1Ch)

	return nil
}

// End terminates the DKG state machine and records the artifacts.
func (c *Controller) End() error {
	state := c.GetState()
	if state != Phase2 {
		return NewInvalidStateTransitionError(state, End)
	}

	c.log.Debug().Msg("DKG engine end")

	// end and retrieve products of the DKG protocol
	c.dkgLock.Lock()
	privateShare, groupPublicKey, publicKeys, err := c.dkg.End()
	c.dkgLock.Unlock()

	if err != nil {
		return err
	}

	c.artifactsLock.Lock()
	c.privateShare = privateShare
	c.groupPublicKey = groupPublicKey
	c.publicKeys = publicKeys
	c.artifactsLock.Unlock()

	c.SetState(End)
	close(c.endCh)

	return nil
}

// Shutdown stops the controller regardless of the current state.
func (c *Controller) Shutdown() {
	c.SetState(Shutdown)
	close(c.shutdownCh)
}

// GetArtifacts returns the private and public shares, as well as the set of
// public keys computed by DKG.
func (c *Controller) GetArtifacts() (crypto.PrivateKey, crypto.PublicKey, []crypto.PublicKey) {
	c.artifactsLock.Lock()
	defer c.artifactsLock.Unlock()
	return c.privateShare, c.groupPublicKey, c.publicKeys
}

/*******************************************************************************
WORKERS
*******************************************************************************/

func (c *Controller) doBackgroundWork() {
	for {
		select {
		case msg := <-c.msgCh:
			c.dkgLock.Lock()
			// The DKG controller doesn't use the epoch and phase values defined
			// in the DKGMessage. These values are used by upstream objects such
			// as the DKG processor engine, and the DKG smart contract.
			err := c.dkg.HandleMsg(msg.Orig, msg.Data)
			if err != nil {
				c.log.Err(err).Msg("Error processing DKG message")
			}
			c.dkgLock.Unlock()
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

	c.dkgLock.Lock()
	err := c.dkg.Start(c.seed)
	c.dkgLock.Unlock()
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
			return nil
		}
	}
}

func (c *Controller) phase1() error {
	state := c.GetState()
	if state != Phase1 {
		return fmt.Errorf("Cannot execute phase1 routine in state %s", state)
	}

	c.dkgLock.Lock()
	err := c.dkg.NextTimeout()
	c.dkgLock.Unlock()
	if err != nil {
		return fmt.Errorf("Error calling NextTimeout: %w", err)
	}

	c.log.Debug().Msg("Waiting for end of phase 1")
	for {
		select {
		case <-c.h1Ch:
			return nil
		case <-c.shutdownCh:
			return nil
		}
	}
}

func (c *Controller) phase2() error {
	state := c.GetState()
	if state != Phase2 {
		return fmt.Errorf("Cannot execute phase2 routine in state %s", state)
	}

	c.dkgLock.Lock()
	err := c.dkg.NextTimeout()
	c.dkgLock.Unlock()
	if err != nil {
		return fmt.Errorf("Error calling NextTimeout: %w", err)
	}

	c.log.Debug().Msg("Waiting for end of phase 2")
	for {
		select {
		case <-c.endCh:
			return nil
		case <-c.shutdownCh:
			return nil
		}
	}
}
