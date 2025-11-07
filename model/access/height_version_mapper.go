package access

import (
	"maps"
	"slices"
)

var LatestBoundary = map[uint64]Version{0: VersionLatest}

// HeightVersionMapper defines the interface for mapping heights to protocol versions.
type HeightVersionMapper interface {
	// GetVersion returns the version corresponding to the given height.
	GetVersion(height uint64) Version

	// All versions defined in the mapper.
	AllVersions() []Version
}

// StaticHeightVersionMapper is an implementation that allows hardcoding the height boundaries
// for each protocol version.
type StaticHeightVersionMapper struct {
	heightVersionBoundaries map[uint64]Version
	boundaries              []uint64
	versions                []Version
}

var _ HeightVersionMapper = (*StaticHeightVersionMapper)(nil)

func NewStaticHeightVersionMapper(heightVersionBoundaries map[uint64]Version) *StaticHeightVersionMapper {
	boundaries := slices.Collect(maps.Keys(heightVersionBoundaries))
	slices.Sort(boundaries)
	slices.Reverse(boundaries)
	versions := slices.Collect(maps.Values(heightVersionBoundaries))

	if len(boundaries) == 0 {
		panic("no height version boundaries provided. must at least include a catch all entry at height 0.")
	}

	if _, ok := heightVersionBoundaries[0]; !ok {
		panic("version mapping must include a catch all entry at height 0")
	}

	return &StaticHeightVersionMapper{
		heightVersionBoundaries: heightVersionBoundaries,
		boundaries:              boundaries,
		versions:                versions,
	}
}

func (s *StaticHeightVersionMapper) GetVersion(height uint64) Version {
	for _, boundary := range s.boundaries {
		if height >= boundary {
			return s.heightVersionBoundaries[boundary]
		}
	}
	// since the constructor requires that a boundary exists at 0, it should not be possible to get here.
	// this indicates there is a bug.
	panic("height is before any known version boundary")
}

func (s *StaticHeightVersionMapper) AllVersions() []Version {
	return s.versions
}
