package event

import (
	"github.com/onflow/cadence"

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
	serviceAccount := chain.ServiceAddress().String()
	if event.EventType.EventTypeID.Location != serviceAccount {
		return false
	}
	_, has := serviceEventWhitelist[event.EventType.EventTypeID.Identifier]
	return has
}
