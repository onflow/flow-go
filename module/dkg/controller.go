package dkg

import (
	"fmt"
	"sync"

	"github.com/onflow/crypto"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// Controller implements the DKGController interface. It controls the execution
// of a Joint Feldman DKG instance. A new Controller must be instantiated for
// every epoch.
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
	h1Ch       chan struct{}
	h2Ch       chan struct{}
	endCh      chan struct{}
	shutdownCh chan struct{}

	// private fields that hold the DKG artifacts when the protocol runs to
	// completion
	privateShare   crypto.PrivateKey
	publicKeys     []crypto.PublicKey
	groupPublicKey crypto.PublicKey

	// artifactsLock protects access to artifacts
	artifactsLock sync.Mutex

	once *sync.Once
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
		Str("component", "dkg_controller").
		Str("dkg_instance_id", dkgInstanceID).
		Logger()

	return &Controller{
		log:        logger,
		dkg:        dkg,
		seed:       seed,
		broker:     broker,
		h1Ch:       make(chan struct{}),
		h2Ch:       make(chan struct{}),
		endCh:      make(chan struct{}),
		shutdownCh: make(chan struct{}),
		once:       new(sync.Once),
	}
}

/*******************************************************************************
Implement DKGController
*******************************************************************************/

// Run starts the DKG controller and executes the DKG state-machine. It blocks
// until the controller is shutdown or until an error is encountered in one of
// the protocol phases.
func (c *Controller) Run() error {

	// Start DKG and transition to phase 1
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
		case Phase3:
			err := c.phase3()
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

// EndPhase2 notifies the controller to end phase 2, and start phase 3
func (c *Controller) EndPhase2() error {
	state := c.GetState()
	if state != Phase2 {
		return NewInvalidStateTransitionError(state, Phase3)
	}

	c.SetState(Phase3)
	close(c.h2Ch)

	return nil
}

// End terminates the DKG state machine and records the artifacts.
func (c *Controller) End() error {
	state := c.GetState()
	if state != Phase3 {
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
	c.broker.Shutdown()
	c.SetState(Shutdown)
	close(c.shutdownCh)
}

// Poll instructs the broker to read new broadcast messages, which will be
// relayed through the message channel. The function does not return until the
// received messages are processed.
func (c *Controller) Poll(blockReference flow.Identifier) error {
	return c.broker.Poll(blockReference)
}

// GetArtifacts returns our node's private key share, the group public key,
// and the list of all nodes' public keys (including ours), as computed by
// the DKG.
func (c *Controller) GetArtifacts() (crypto.PrivateKey, crypto.PublicKey, []crypto.PublicKey) {
	c.artifactsLock.Lock()
	defer c.artifactsLock.Unlock()
	return c.privateShare, c.groupPublicKey, c.publicKeys
}

// GetIndex returns the index of this node in the DKG committee list.
func (c *Controller) GetIndex() int {
	return c.broker.GetIndex()
}

// SubmitResult instructs the broker to submit DKG results. It is up to the
// caller to ensure that this method is called after a succesfull run of the
// protocol.
func (c *Controller) SubmitResult() error {
	_, pubKey, groupKeys := c.GetArtifacts()
	return c.broker.SubmitResult(pubKey, groupKeys)
}

/*******************************************************************************
WORKERS
*******************************************************************************/

func (c *Controller) doBackgroundWork() {
	privateMsgCh := c.broker.GetPrivateMsgCh()
	broadcastMsgCh := c.broker.GetBroadcastMsgCh()
	for {
		select {
		case msg := <-privateMsgCh:
			c.dkgLock.Lock()
			err := c.dkg.HandlePrivateMsg(int(msg.CommitteeMemberIndex), msg.Data)
			c.dkgLock.Unlock()
			if err != nil {
				c.log.Err(err).Msg("error processing DKG private message")
			}

		case msg := <-broadcastMsgCh:

			c.dkgLock.Lock()
			err := c.dkg.HandleBroadcastMsg(int(msg.CommitteeMemberIndex), msg.Data)
			c.dkgLock.Unlock()
			if err != nil {
				c.log.Err(err).Msg("error processing DKG broadcast message")
			}

		case <-c.shutdownCh:
			return
		}
	}
}

func (c *Controller) start() error {
	state := c.GetState()
	if state != Init {
		return fmt.Errorf("cannot execute start routine in state %s", state)
	}

	c.dkgLock.Lock()
	err := c.dkg.Start(c.seed)
	c.dkgLock.Unlock()
	if err != nil {
		return fmt.Errorf("Error starting DKG: %w", err)
	}

	c.log.Debug().Msg("DKG engine started")
	c.SetState(Phase1)
	return nil
}

func (c *Controller) phase1() error {
	state := c.GetState()
	if state != Phase1 {
		return fmt.Errorf("Cannot execute phase1 routine in state %s", state)
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
		case <-c.h2Ch:
			return nil
		case <-c.shutdownCh:
			return nil
		}
	}
}

func (c *Controller) phase3() error {
	state := c.GetState()
	if state != Phase3 {
		return fmt.Errorf("Cannot execute phase3 routine in state %s", state)
	}

	c.dkgLock.Lock()
	err := c.dkg.NextTimeout()
	c.dkgLock.Unlock()
	if err != nil {
		return fmt.Errorf("Error calling NextTimeout: %w", err)
	}

	c.log.Debug().Msg("Waiting for end of phase 3")
	for {
		select {
		case <-c.endCh:
			return nil
		case <-c.shutdownCh:
			return nil
		}
	}
}
