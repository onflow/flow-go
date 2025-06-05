package chunks

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// Locator is used to locate a chunk by providing the execution result the chunk belongs to as well as the chunk index within that execution result.
// Since a chunk is unique by the result ID and its index in the result's chunk list.
//
//structwrite:immutable - mutations allowed only within the constructor
type Locator struct {
	ResultID flow.Identifier // execution result id that chunk belongs to
	Index    uint64          // index of chunk in the execution result
}

// UntrustedLocator is an untrusted input-only representation of a Locator,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedLocator should be validated and converted into
// a trusted Locator using NewLocator constructor.
type UntrustedLocator Locator

// NewEpochStateContainer creates a new instance of Locator.
// Construction Locator allowed only within the constructor.
//
// All errors indicate a valid Locator cannot be constructed from the input.

// NewLocator creates a new instance of Locator.
// Construction Locator allowed only within the constructor.
func NewLocator(untrusted UntrustedLocator) (*Locator, error) {
	if untrusted.ResultID == flow.ZeroID {
		return nil, fmt.Errorf("ResultID must not be zero")
	}
	return &Locator{
		ResultID: untrusted.ResultID,
		Index:    untrusted.Index,
	}, nil
}

// ID returns a unique id for chunk locator.
func (c Locator) ID() flow.Identifier {
	return flow.MakeID(c)
}

// Checksum provides a cryptographic commitment for a chunk locator content.
func (c Locator) Checksum() flow.Identifier {
	return flow.MakeID(c)
}

// LocatorMap maps keeps chunk locators based on their locator id.
type LocatorMap map[flow.Identifier]*Locator

func (l LocatorMap) ToList() LocatorList {
	locatorList := LocatorList{}
	for _, locator := range l {
		locatorList = append(locatorList, locator)
	}

	return locatorList
}

type LocatorList []*Locator

func (l LocatorList) ToMap() LocatorMap {
	locatorMap := make(LocatorMap)
	for _, locator := range l {
		locatorMap[locator.ID()] = locator
	}
	return locatorMap
}
