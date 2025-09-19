package fixtures

import "github.com/onflow/flow-go/model/flow"

// ProtocolStateVersionUpgrade is the default options factory for [flow.ProtocolStateVersionUpgrade] generation.
var ProtocolStateVersionUpgrade protocolStateVersionUpgradeFactory

type protocolStateVersionUpgradeFactory struct{}

type ProtocolStateVersionUpgradeOption func(*ProtocolStateVersionUpgradeGenerator, *flow.ProtocolStateVersionUpgrade)

// WithNewProtocolStateVersion is an option that sets the `NewProtocolStateVersion` of the upgrade.
func (f protocolStateVersionUpgradeFactory) WithNewProtocolStateVersion(version uint64) ProtocolStateVersionUpgradeOption {
	return func(g *ProtocolStateVersionUpgradeGenerator, upgrade *flow.ProtocolStateVersionUpgrade) {
		upgrade.NewProtocolStateVersion = version
	}
}

// WithActiveView is an option that sets the `ActiveView` of the upgrade.
func (f protocolStateVersionUpgradeFactory) WithActiveView(view uint64) ProtocolStateVersionUpgradeOption {
	return func(g *ProtocolStateVersionUpgradeGenerator, upgrade *flow.ProtocolStateVersionUpgrade) {
		upgrade.ActiveView = view
	}
}

// ProtocolStateVersionUpgradeGenerator generates protocol state version upgrades with consistent randomness.
type ProtocolStateVersionUpgradeGenerator struct {
	protocolStateVersionUpgradeFactory

	random *RandomGenerator
}

// NewProtocolStateVersionUpgradeGenerator creates a new ProtocolStateVersionUpgradeGenerator.
func NewProtocolStateVersionUpgradeGenerator(random *RandomGenerator) *ProtocolStateVersionUpgradeGenerator {
	return &ProtocolStateVersionUpgradeGenerator{
		random: random,
	}
}

// Fixture generates a [flow.ProtocolStateVersionUpgrade] with random data based on the provided options.
func (g *ProtocolStateVersionUpgradeGenerator) Fixture(opts ...ProtocolStateVersionUpgradeOption) *flow.ProtocolStateVersionUpgrade {
	upgrade := &flow.ProtocolStateVersionUpgrade{
		NewProtocolStateVersion: g.random.Uint64(),
		ActiveView:              g.random.Uint64(),
	}

	for _, opt := range opts {
		opt(g, upgrade)
	}

	return upgrade
}

// List generates a list of [flow.ProtocolStateVersionUpgrade].
func (g *ProtocolStateVersionUpgradeGenerator) List(n int, opts ...ProtocolStateVersionUpgradeOption) []*flow.ProtocolStateVersionUpgrade {
	upgrades := make([]*flow.ProtocolStateVersionUpgrade, n)
	for i := range n {
		upgrades[i] = g.Fixture(opts...)
	}
	return upgrades
}
