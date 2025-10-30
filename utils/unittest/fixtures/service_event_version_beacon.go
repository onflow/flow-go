package fixtures

import (
	"fmt"

	"github.com/coreos/go-semver/semver"

	"github.com/onflow/flow-go/model/flow"
)

// VersionBeacon is the default options factory for [flow.VersionBeacon] generation.
var VersionBeacon versionBeaconFactory

type versionBeaconFactory struct{}

type VersionBeaconOption func(*VersionBeaconGenerator, *flow.VersionBeacon)

// WithSequence is an option that sets the `Sequence` of the version beacon.
func (f versionBeaconFactory) WithSequence(sequence uint64) VersionBeaconOption {
	return func(g *VersionBeaconGenerator, vb *flow.VersionBeacon) {
		vb.Sequence = sequence
	}
}

// WithBoundaries is an option that sets the `VersionBoundaries` of the version beacon.
func (f versionBeaconFactory) WithBoundaries(boundaries ...flow.VersionBoundary) VersionBeaconOption {
	return func(g *VersionBeaconGenerator, vb *flow.VersionBeacon) {
		vb.VersionBoundaries = boundaries
	}
}

// VersionBeaconGenerator generates version beacons with consistent randomness.
type VersionBeaconGenerator struct {
	versionBeaconFactory

	random *RandomGenerator
}

// NewVersionBeaconGenerator creates a new VersionBeaconGenerator.
func NewVersionBeaconGenerator(random *RandomGenerator) *VersionBeaconGenerator {
	return &VersionBeaconGenerator{
		random: random,
	}
}

// Fixture generates a [flow.VersionBeacon] with random data based on the provided options.
func (g *VersionBeaconGenerator) Fixture(opts ...VersionBeaconOption) *flow.VersionBeacon {
	numBoundaries := g.random.IntInRange(1, 4)

	height := g.random.Uint64InRange(0, 1000)
	version := g.generateRandomVersion()

	boundaries := make([]flow.VersionBoundary, numBoundaries)
	for i := range numBoundaries {
		boundaries[i] = flow.VersionBoundary{
			Version:     version.String(),
			BlockHeight: height,
		}

		// increment the version so boundaries are always increasing
		if i%3 == 0 {
			version.BumpMajor()
		} else if i%2 == 0 {
			version.BumpMinor()
		} else {
			version.BumpPatch()
		}
		height += g.random.Uint64InRange(1000, 100_000)
	}

	vb := &flow.VersionBeacon{
		VersionBoundaries: boundaries,
		Sequence:          uint64(g.random.Uint32()),
	}

	for _, opt := range opts {
		opt(g, vb)
	}

	return vb
}

// List generates a list of [flow.VersionBeacon].
func (g *VersionBeaconGenerator) List(n int, opts ...VersionBeaconOption) []*flow.VersionBeacon {
	beacons := make([]*flow.VersionBeacon, n)
	for i := range n {
		beacons[i] = g.Fixture(opts...)
	}
	return beacons
}

// generateRandomVersion creates a random semver version string.
func (g *VersionBeaconGenerator) generateRandomVersion() *semver.Version {
	major := g.random.IntInRange(0, 5)
	minor := g.random.IntInRange(0, 20)
	patch := g.random.IntInRange(0, 50)
	return semver.New(fmt.Sprintf("%d.%d.%d", major, minor, patch))
}
