package access

import (
	"fmt"
	"maps"
	"slices"
)

type HeightVersionMapper interface {
	GetVersion(height uint64) (string, error)
	VersionExists(version string) bool
}

type StaticHeightVersionMapper struct {
	heightVersionBoundaries map[uint64]string
	boundaries              []uint64
}

func NewStaticHeightVersionMapper(heightVersionBoundaries map[uint64]string) *StaticHeightVersionMapper {
	boundaries := slices.Collect(maps.Keys(heightVersionBoundaries))
	slices.Sort(boundaries)
	slices.Reverse(boundaries)

	return &StaticHeightVersionMapper{
		heightVersionBoundaries: heightVersionBoundaries,
		boundaries:              boundaries,
	}
}

func (s *StaticHeightVersionMapper) GetVersion(height uint64) (string, error) {
	for _, boundary := range s.boundaries {
		if height >= boundary {
			return s.heightVersionBoundaries[boundary], nil
		}
	}

	return "", fmt.Errorf("height %d is before any known version boundary", height)
}

func (s *StaticHeightVersionMapper) VersionExists(version string) bool {
	for _, v := range s.heightVersionBoundaries {
		if v == version {
			return true
		}
	}
	return false
}
