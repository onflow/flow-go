package dkg

import (
	"fmt"

	"github.com/onflow/crypto"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/signature"
)

// ControllerFactory is a factory object that creates new Controllers for new
// epochs. Each Controller produced by a factory shares the same underlying
// Local object to sign broadcast messages, the same tunnel tying it to the
// MessagingEngine, and the same client to communicate with the DKG
// smart-contract.
type ControllerFactory struct {
	log                zerolog.Logger
	me                 module.Local
	dkgContractClients []module.DKGContractClient
	tunnel             *BrokerTunnel
}

// NewControllerFactory creates a new factory that generates Controllers with
// the same underlying Local object, tunnel and dkg smart-contract client.
func NewControllerFactory(
	log zerolog.Logger,
	me module.Local,
	dkgContractClients []module.DKGContractClient,
	tunnel *BrokerTunnel,
) *ControllerFactory {

	return &ControllerFactory{
		log:                log,
		me:                 me,
		dkgContractClients: dkgContractClients,
		tunnel:             tunnel,
	}
}

// Create creates a new epoch-specific Controller equipped with a broker which
// is capable of communicating with other nodes.
func (f *ControllerFactory) Create(
	dkgInstanceID string,
	participants flow.IdentitySkeletonList,
	seed []byte) (module.DKGController, error) {

	myIndex, ok := participants.GetIndex(f.me.NodeID())
	if !ok {
		return nil, fmt.Errorf("failed to create controller factory, node %s is not part of DKG committee", f.me.NodeID().String())
	}

	broker := NewBroker(
		f.log,
		dkgInstanceID,
		participants,
		f.me,
		int(myIndex),
		f.dkgContractClients,
		f.tunnel,
	)

	n := len(participants)
	threshold := signature.RandomBeaconThreshold(n)
	dkg, err := crypto.NewJointFeldman(n, threshold, int(myIndex), broker)
	if err != nil {
		return nil, err
	}

	controller := NewController(
		f.log,
		dkgInstanceID,
		dkg,
		seed,
		broker,
	)

	return controller, nil
}
