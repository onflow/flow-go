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
		&NodeRecord{id: 1},
		&NodeRecord{id: 3},
		&NodeRecord{id: 2},
		&NodeRecord{id: 5},
	}

	sort.Sort(nodes)

	Expect(nodes).To(Equal(
		NodeRecords{
			&NodeRecord{id: 1},
			&NodeRecord{id: 2},
			&NodeRecord{id: 3},
			&NodeRecord{id: 5},
		},
	))
}

func TestNodeIdentitiesSort(t *testing.T) {
	RegisterTestingT(t)

	nodes := nodeIdentities{
		&nodeIdentity{NodeRecord: &NodeRecord{id: 1}, index:11},
		&nodeIdentity{NodeRecord: &NodeRecord{id: 3}, index:33},
		&nodeIdentity{NodeRecord: &NodeRecord{id: 2}, index:22},
		&nodeIdentity{NodeRecord: &NodeRecord{id: 5}, index:55},
	}

	sort.Sort(nodes)

	Expect(nodes).To(Equal(
		nodeIdentities{
			&nodeIdentity{NodeRecord: &NodeRecord{id: 1}, index:11},
			&nodeIdentity{NodeRecord: &NodeRecord{id: 2}, index:22},
			&nodeIdentity{NodeRecord: &NodeRecord{id: 3}, index:33},
			&nodeIdentity{NodeRecord: &NodeRecord{id: 5}, index:55},
		},
	))
}

func TestNewInMemoryIdentityTable(t *testing.T) {
	RegisterTestingT(t)

	nodes := nodeIdentities{
		&nodeIdentity{NodeRecord: &NodeRecord{id: 1}, index:11},
		&nodeIdentity{NodeRecord: &NodeRecord{id: 3}, index:33},
		&nodeIdentity{NodeRecord: &NodeRecord{id: 2}, index:22},
		&nodeIdentity{NodeRecord: &NodeRecord{id: 5}, index:55},
	}

	sort.Sort(nodes)

	Expect(nodes).To(Equal(
		nodeIdentities{
			&nodeIdentity{NodeRecord: &NodeRecord{id: 1}, index:11},
			&nodeIdentity{NodeRecord: &NodeRecord{id: 2}, index:22},
			&nodeIdentity{NodeRecord: &NodeRecord{id: 3}, index:33},
			&nodeIdentity{NodeRecord: &NodeRecord{id: 5}, index:55},
		},
	))
}


