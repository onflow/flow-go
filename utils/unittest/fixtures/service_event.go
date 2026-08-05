package fixtures

import (
	"github.com/onflow/flow-go/model/flow"
)

// ServiceEvent is the default options factory for [flow.ServiceEvent] generation.
var ServiceEvent serviceEventFactory

type serviceEventFactory struct{}

type ServiceEventOption func(*ServiceEventGenerator, *flow.ServiceEvent)

// WithType is an option that sets the `Type` of the service event.
func (f serviceEventFactory) WithType(eventType flow.ServiceEventType) ServiceEventOption {
	return func(g *ServiceEventGenerator, event *flow.ServiceEvent) {
		event.Type = eventType
	}
}

// WithEvent is an option that sets the `Event` data of the service event.
func (f serviceEventFactory) WithEvent(eventData interface{}) ServiceEventOption {
	return func(g *ServiceEventGenerator, event *flow.ServiceEvent) {
		event.Event = eventData
	}
}

// ServiceEventGenerator generates service events with consistent randomness.
type ServiceEventGenerator struct {
	serviceEventFactory

	random                       *RandomGenerator
	epochSetups                  *EpochSetupGenerator
	epochCommits                 *EpochCommitGenerator
	epochRecovers                *EpochRecoverGenerator
	versionBeacons               *VersionBeaconGenerator
	protocolStateVersionUpgrades *ProtocolStateVersionUpgradeGenerator
	setEpochExtensionViewCounts  *SetEpochExtensionViewCountGenerator
	ejectNodes                   *EjectNodeGenerator
}

// NewServiceEventGenerator creates a new ServiceEventGenerator.
func NewServiceEventGenerator(
	random *RandomGenerator,
	epochSetups *EpochSetupGenerator,
	epochCommits *EpochCommitGenerator,
	epochRecovers *EpochRecoverGenerator,
	versionBeacons *VersionBeaconGenerator,
	protocolStateVersionUpgrades *ProtocolStateVersionUpgradeGenerator,
	setEpochExtensionViewCounts *SetEpochExtensionViewCountGenerator,
	ejectNodes *EjectNodeGenerator,
) *ServiceEventGenerator {
	return &ServiceEventGenerator{
		random:                       random,
		epochSetups:                  epochSetups,
		epochCommits:                 epochCommits,
		epochRecovers:                epochRecovers,
		versionBeacons:               versionBeacons,
		protocolStateVersionUpgrades: protocolStateVersionUpgrades,
		setEpochExtensionViewCounts:  setEpochExtensionViewCounts,
		ejectNodes:                   ejectNodes,
	}
}

// Fixture generates a [flow.ServiceEvent] with random data based on the provided options.
func (g *ServiceEventGenerator) Fixture(opts ...ServiceEventOption) flow.ServiceEvent {
	// Define available service event types
	eventTypes := []flow.ServiceEventType{
		flow.ServiceEventSetup,
		flow.ServiceEventCommit,
		flow.ServiceEventRecover,
		flow.ServiceEventVersionBeacon,
		flow.ServiceEventProtocolStateVersionUpgrade,
		flow.ServiceEventSetEpochExtensionViewCount,
		flow.ServiceEventEjectNode,
	}

	// Pick a random event type
	eventType := RandomElement(g.random, eventTypes)

	event := flow.ServiceEvent{
		Type: eventType,
	}

	// Apply options first (may change the type)
	for _, opt := range opts {
		opt(g, &event)
	}

	// Generate event data based on the final type using specific generators
	switch event.Type {
	case flow.ServiceEventSetup:
		event.Event = g.epochSetups.Fixture()
	case flow.ServiceEventCommit:
		event.Event = g.epochCommits.Fixture()
	case flow.ServiceEventRecover:
		event.Event = g.epochRecovers.Fixture()
	case flow.ServiceEventVersionBeacon:
		event.Event = g.versionBeacons.Fixture()
	case flow.ServiceEventProtocolStateVersionUpgrade:
		event.Event = g.protocolStateVersionUpgrades.Fixture()
	case flow.ServiceEventSetEpochExtensionViewCount:
		event.Event = g.setEpochExtensionViewCounts.Fixture()
	case flow.ServiceEventEjectNode:
		event.Event = g.ejectNodes.Fixture()
	default:
		Assertf(false, "unexpected service event type: %s", event.Type)
	}

	return event
}

// List generates a list of [flow.ServiceEvent].
func (g *ServiceEventGenerator) List(n int, opts ...ServiceEventOption) []flow.ServiceEvent {
	events := make([]flow.ServiceEvent, n)
	for i := range n {
		events[i] = g.Fixture(opts...)
	}
	return events
}
