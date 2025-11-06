package access

import (
	"fmt"
	"maps"
	"slices"
)

type Version int8

var (
	VersionV0 Version = 0
	VersionV1 Version = 1
	VersionV2 Version = 2
)

type Versioned[T any] struct {
	versionedTypes map[Version]T
	versionMapper  HeightVersionMapper
}

func NewVersioned[T any](versionedTypes map[Version]T, versionMapper HeightVersionMapper) (*Versioned[T], error) {
	for _, ver := range slices.Collect(maps.Keys(versionedTypes)) {
		if !versionMapper.VersionExists(ver) {
			return nil, fmt.Errorf("version missing in the version mapper: %v", ver)
		}
	}

	return &Versioned[T]{
		versionedTypes: versionedTypes,
		versionMapper:  versionMapper,
	}, nil
}

func (v *Versioned[T]) Get(height uint64) T {
	version, err := v.versionMapper.GetVersion(height)
	t, ok := v.versionedTypes[version]
	if err != nil || !ok {
		return v.versionedTypes[VersionV2] // latest
	}

	return t
}

func (v *Versioned[T]) all() []T {
	return slices.Collect(maps.Values(v.versionedTypes))
}
