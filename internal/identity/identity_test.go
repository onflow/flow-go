package identity

import (
	"sort"
	"testing"

	. "github.com/onsi/gomega"
)

// Compile-time check that nodeIdentity implements interface identity.NodeIdentity
func TestNodeIdentitiesImplementsInterface(t *testing.T) {
	var _ NodeIdentity = (*nodeIdentity)(nil)
}

// Compile-time check that InMemoryIdentityTable implements interface identity.Table
func TestInMemoryIdentityTableImplementsTable(t *testing.T) {
	var _ Table = (*InMemoryIdentityTable)(nil)
}

// Compile-time check that []nodeIdentity, aka nodeIdentities, implements interface sort.Interface
func TestNodeIdentitiesImplementsSortable(t *testing.T) {
	var _ sort.Interface = (*nodeIdentities)(nil)
}

// Compile-time check that []NodeRecord, aka NodeRecords, implements interface sort.Interface
func TestNodeRecordsImplementsSortable(t *testing.T) {
	var _ sort.Interface = (*NodeRecords)(nil)
}

func TestNodeRecordsSort(t *testing.T) {
	RegisterTestingT(t)

	nodes := NodeRecords{
		&NodeRecord{ID: 1},
		&NodeRecord{ID: 3},
		&NodeRecord{ID: 2},
		&NodeRecord{ID: 5},
	}

	sort.Sort(nodes)

	Expect(nodes).To(Equal(
		NodeRecords{
			&NodeRecord{ID: 1},
			&NodeRecord{ID: 2},
			&NodeRecord{ID: 3},
			&NodeRecord{ID: 5},
		},
	))
}

func TestNodeIdentitiesSort(t *testing.T) {
	RegisterTestingT(t)

	nodes := nodeIdentities{
		&nodeIdentity{coreID: &NodeRecord{ID: 1}, index:11},
		&nodeIdentity{coreID: &NodeRecord{ID: 3}, index:33},
		&nodeIdentity{coreID: &NodeRecord{ID: 2}, index:22},
		&nodeIdentity{coreID: &NodeRecord{ID: 5}, index:55},
	}

	sort.Sort(nodes)

	Expect(nodes).To(Equal(
		nodeIdentities{
			&nodeIdentity{coreID: &NodeRecord{ID: 1}, index:11},
			&nodeIdentity{coreID: &NodeRecord{ID: 2}, index:22},
			&nodeIdentity{coreID: &NodeRecord{ID: 3}, index:33},
			&nodeIdentity{coreID: &NodeRecord{ID: 5}, index:55},
		},
	))
}

func TestNewInMemoryIdentityTable(t *testing.T) {
	RegisterTestingT(t)

	nodes := nodeIdentities{
		&nodeIdentity{coreID: &NodeRecord{ID: 1}, index:11},
		&nodeIdentity{coreID: &NodeRecord{ID: 3}, index:33},
		&nodeIdentity{coreID: &NodeRecord{ID: 2}, index:22},
		&nodeIdentity{coreID: &NodeRecord{ID: 5}, index:55},
	}

	sort.Sort(nodes)

	Expect(nodes).To(Equal(
		nodeIdentities{
			&nodeIdentity{coreID: &NodeRecord{ID: 1}, index:11},
			&nodeIdentity{coreID: &NodeRecord{ID: 2}, index:22},
			&nodeIdentity{coreID: &NodeRecord{ID: 3}, index:33},
			&nodeIdentity{coreID: &NodeRecord{ID: 5}, index:55},
		},
	))
}


