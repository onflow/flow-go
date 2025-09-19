package fixtures

import "github.com/onflow/flow-go/model/flow"

// SetEpochExtensionViewCount is the default options factory for [flow.SetEpochExtensionViewCount] generation.
var SetEpochExtensionViewCount setEpochExtensionViewCountFactory

type setEpochExtensionViewCountFactory struct{}

type SetEpochExtensionViewCountOption func(*SetEpochExtensionViewCountGenerator, *flow.SetEpochExtensionViewCount)

// WithValue is an option that sets the `Value` of the extension view count.
func (f setEpochExtensionViewCountFactory) WithValue(value uint64) SetEpochExtensionViewCountOption {
	return func(g *SetEpochExtensionViewCountGenerator, extension *flow.SetEpochExtensionViewCount) {
		extension.Value = value
	}
}

// SetEpochExtensionViewCountGenerator generates epoch extension view count events with consistent randomness.
type SetEpochExtensionViewCountGenerator struct {
	setEpochExtensionViewCountFactory

	random *RandomGenerator
}

// NewSetEpochExtensionViewCountGenerator creates a new SetEpochExtensionViewCountGenerator.
func NewSetEpochExtensionViewCountGenerator(random *RandomGenerator) *SetEpochExtensionViewCountGenerator {
	return &SetEpochExtensionViewCountGenerator{
		random: random,
	}
}

// Fixture generates a [flow.SetEpochExtensionViewCount] with random data based on the provided options.
func (g *SetEpochExtensionViewCountGenerator) Fixture(opts ...SetEpochExtensionViewCountOption) *flow.SetEpochExtensionViewCount {
	extension := &flow.SetEpochExtensionViewCount{
		Value: g.random.Uint64InRange(200, 10000),
	}

	for _, opt := range opts {
		opt(g, extension)
	}

	return extension
}

// List generates a list of [flow.SetEpochExtensionViewCount].
func (g *SetEpochExtensionViewCountGenerator) List(n int, opts ...SetEpochExtensionViewCountOption) []*flow.SetEpochExtensionViewCount {
	extensions := make([]*flow.SetEpochExtensionViewCount, n)
	for i := range n {
		extensions[i] = g.Fixture(opts...)
	}
	return extensions
}
