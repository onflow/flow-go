package access

import (
	"fmt"
	"maps"
	"slices"
)

type HeightVersionMapper interface {
	GetVersion(height uint64) (Version, error)
	VersionExists(version Version) bool
}

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
