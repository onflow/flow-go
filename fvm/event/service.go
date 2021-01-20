package event

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/model/flow"
)

// Keep serviceEventWhitelist module-only to prevent accidental modifications
var serviceEventWhitelist = map[string]struct{}{
	"EpochManager.EpochSetup": {},
}

var serviceEventWhitelistFlat []string

func init() {
	for s := range serviceEventWhitelist {
		serviceEventWhitelistFlat = append(serviceEventWhitelistFlat, s)
	}
}

func GetServiceEventWhitelist() []string {
	return serviceEventWhitelistFlat
}

func IsServiceEvent(event cadence.Event, chain flow.Chain) bool {
	serviceAccount := chain.ServiceAddress()

	addressLocation, casted := event.EventType.Location.(common.AddressLocation)
	if !casted {
		return false
	}

	flowAddress := flow.BytesToAddress(addressLocation.Address.Bytes())

	if flowAddress != serviceAccount {
		return false
	}
	_, has := serviceEventWhitelist[event.EventType.QualifiedIdentifier]
	return has
}
