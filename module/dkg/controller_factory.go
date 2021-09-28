package dkg

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/crypto"
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
	tunnel *BrokerTunnel) *ControllerFactory {

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
	participants flow.IdentityList,
	seed []byte) (module.DKGController, error) {

	myIndex := -1
	for i, id := range participants.NodeIDs() {
		if id == f.me.NodeID() {
			myIndex = i
			break
		}
	}
	if myIndex < 0 {
		return nil, fmt.Errorf("node does not belong to dkg committee")
	}

	broker := NewBroker(
		f.log,
		dkgInstanceID,
		participants,
		f.me,
		myIndex,
		f.dkgContractClients,
		f.tunnel,
	)

	n := len(participants)
	threshold := signature.RandomBeaconThreshold(n)
	dkg, err := crypto.NewJointFeldman(n, threshold, myIndex, broker)
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
