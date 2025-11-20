package access

import (
	"fmt"
	"maps"
	"math"
	"slices"
)

type Version uint8

var (
	VersionLatest Version = math.MaxInt8
)

// Versioned is a generic container that associates different versions of type T with their corresponding protocol versions,
// as determined by the HeightVersionMapper. This allows retaining and retrieving historical implementations of a type,
// ensuring correct behavior when the protocol evolves and previous versions must remain accessible for older data.
type Versioned[T any] struct {
	versionedTypes map[Version]T
	versionMapper  HeightVersionMapper
}

func NewVersioned[T any](versionedTypes map[Version]T, versionMapper HeightVersionMapper) *Versioned[T] {
	for _, ver := range versionMapper.AllVersions() {
		if _, ok := versionedTypes[ver]; !ok {
			// the provided mapping is inconsistent. this is a development time error, so panic.
			panic(fmt.Sprintf("version missing in the version mapper: %v", ver))
		}
	}

	return &Versioned[T]{
		versionedTypes: versionedTypes,
		versionMapper:  versionMapper,
	}
}

// ByHeight version of the type at the provided height.
func (v *Versioned[T]) ByHeight(height uint64) T {
	version := v.versionMapper.GetVersion(height)
	return v.versionedTypes[version]
}

// AllVersions returns All versions defined in the mapper.
// Note: the values are stored within a map, so the order of the returned slice is not deterministic.
func (v *Versioned[T]) All() []T {
	return slices.Collect(maps.Values(v.versionedTypes))
}
