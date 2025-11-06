package access

import (
	"fmt"
	"maps"
	"slices"

	"github.com/onflow/flow-go/model/flow"
)

var latest = map[uint64]Version{0: VersionV2}

var HardcodedChainHeightVersions = map[flow.ChainID]HeightVersionMapper{
	// todo define these
	flow.Mainnet: NewStaticHeightVersionMapper(map[uint64]Version{
		0:   VersionV0,
		200: VersionV1,
		300: VersionV2,
	}),
	flow.Testnet: NewStaticHeightVersionMapper(map[uint64]Version{
		0:   VersionV0,
		200: VersionV1,
		300: VersionV2,
	}),
	flow.Sandboxnet:        NewStaticHeightVersionMapper(latest),
	flow.Previewnet:        NewStaticHeightVersionMapper(latest),
	flow.Benchnet:          NewStaticHeightVersionMapper(latest),
	flow.Localnet:          NewStaticHeightVersionMapper(latest),
	flow.Emulator:          NewStaticHeightVersionMapper(latest),
	flow.BftTestnet:        NewStaticHeightVersionMapper(latest),
	flow.MonotonicEmulator: NewStaticHeightVersionMapper(latest),
}

// HeightVersionMapper defines the interface for mapping heights to protocol versions.
type HeightVersionMapper interface {
	// GetVersion returns the version corresponding to the given height.
	GetVersion(height uint64) (Version, error)
	// VersionExists checks if a version exists in the mapper.
	VersionExists(version Version) bool
}

// StaticHeightVersionMapper is an implementation that allows hardcoding the height boundaries
// for each protocol version.
type StaticHeightVersionMapper struct {
	heightVersionBoundaries map[uint64]Version
	boundaries              []uint64
}

func NewStaticHeightVersionMapper(heightVersionBoundaries map[uint64]Version) *StaticHeightVersionMapper {
	boundaries := slices.Collect(maps.Keys(heightVersionBoundaries))
	slices.Sort(boundaries)
	slices.Reverse(boundaries)

	return &StaticHeightVersionMapper{
		heightVersionBoundaries: heightVersionBoundaries,
		boundaries:              boundaries,
	}
}

func (s *StaticHeightVersionMapper) GetVersion(height uint64) (Version, error) {
	for _, boundary := range s.boundaries {
		if height >= boundary {
			return s.heightVersionBoundaries[boundary], nil
		}
	}

	return 0, fmt.Errorf("height %d is before any known version boundary", height)
}

func (s *StaticHeightVersionMapper) VersionExists(version Version) bool {
	for _, v := range s.heightVersionBoundaries {
		if v == version {
			return true
		}
	}
	return false
}
