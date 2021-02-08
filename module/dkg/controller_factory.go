package dkg

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

type ControllerFactory struct {
	log               zerolog.Logger
	dkgContractClient module.DKGContractClient
	tunnel            *BrokerTunnel
}

func NewControllerFactory(
	log zerolog.Logger,
	dkgContractClient module.DKGContractClient,
	tunnel *BrokerTunnel) *ControllerFactory {

	return &ControllerFactory{
		log:               log,
		dkgContractClient: dkgContractClient,
		tunnel:            tunnel,
	}
}

func (f *ControllerFactory) Create(
	dkgInstanceID string,
	committee flow.IdentifierList,
	myIndex int,
	seed []byte) (*Controller, error) {

	broker := NewBroker(
		f.log,
		dkgInstanceID,
		committee,
		myIndex,
		f.dkgContractClient,
		f.tunnel,
	)

	n := len(committee)
	dkg, err := crypto.NewJointFeldman(n, optimalThreshold(n), myIndex, broker)
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

// optimal threshold (t) to allow the largest number of malicious nodes (m)
// assuming the protocol requires:
//   m<=t for unforgeability
//   n-m>=t+1 for robustness
func optimalThreshold(size int) int {
	return (size - 1) / 2
}
