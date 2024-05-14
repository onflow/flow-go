package derived

import (
	"github.com/onflow/cadence/runtime/common"
	"golang.org/x/exp/maps"
)

// ProgramDependencies are the locations of programs that a program depends on.
type ProgramDependencies struct {
	locations map[common.Location]struct{}
	addresses map[common.Address]struct{}
}

func NewProgramDependencies() ProgramDependencies {
	return ProgramDependencies{
		locations: map[common.Location]struct{}{},
		addresses: map[common.Address]struct{}{},
	}
}

// Count returns the number of locations dependencies of this program.
func (d ProgramDependencies) Count() int {
	return len(d.locations)
}

// Add adds the location as a dependency.
func (d ProgramDependencies) Add(location common.Location) ProgramDependencies {
	d.locations[location] = struct{}{}

	if addressLocation, ok := location.(common.AddressLocation); ok {
		d.addresses[addressLocation.Address] = struct{}{}
	}

	return d
}

// Merge merges current dependencies with other dependencies.
func (d ProgramDependencies) Merge(other ProgramDependencies) {
	for loc := range other.locations {
		d.locations[loc] = struct{}{}
	}
	for address := range other.addresses {
		d.addresses[address] = struct{}{}
	}
}

// ContainsAddress returns true if the address is a dependency.
func (d ProgramDependencies) ContainsAddress(address common.Address) bool {
	_, ok := d.addresses[address]
	return ok
}

// ContainsLocation returns true if the location is a dependency.
func (d ProgramDependencies) ContainsLocation(location common.Location) bool {
	_, ok := d.locations[location]
	return ok
}

// Locations returns all the locations.
func (d ProgramDependencies) Locations() []common.Location {
	return maps.Keys(d.locations)
}
