package dkg

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/module"
)

// Controller implements the DKGController interface. It controls the execution
// of a Joint Feldman DKG instance.
type Controller struct {
	// The embedded state Manager is used to manage the controller's underlying
	// state.
	Manager

	log zerolog.Logger

	// DKGState is the object that actually executes the protocol steps.
	dkg crypto.DKGState

	// dkgLock protects access to dkg
	dkgLock sync.Mutex

	// seed is required by DKGState
	seed []byte

	// broker enables the controller to communicate with other nodes
	broker module.DKGBroker

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
}

// NewController instantiates a new Joint Feldman DKG controller.
func NewController(
	log zerolog.Logger,
	dkgInstanceID string,
	dkg crypto.DKGState,
	seed []byte,
	broker module.DKGBroker,
) *Controller {

	logger := log.With().
		Str("component", "controller").
		Str("dkg_instance_id", dkgInstanceID).
		Logger()

	return &Controller{
		log:        logger,
		dkg:        dkg,
		seed:       seed,
		broker:     broker,
		h0Ch:       make(chan struct{}),
		h1Ch:       make(chan struct{}),
		endCh:      make(chan struct{}),
		shutdownCh: make(chan struct{}),
	}
}

/*******************************************************************************
Implement DKGController
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
	// msgCh is the channel through which the broker forwards incoming messages
	// (private and broadcast). Integrity checks are performed upstream by the
	// broker.
	msgCh := c.broker.GetMsgCh()
	for {
		select {
		case msg := <-msgCh:
			c.dkgLock.Lock()
			err := c.dkg.HandleMsg(msg.Orig, msg.Data)
			c.dkgLock.Unlock()
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
