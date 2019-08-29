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
		&nodeIdentity{NodeRecord: &NodeRecord{id: 1}, index: 11},
		&nodeIdentity{NodeRecord: &NodeRecord{id: 3}, index: 33},
		&nodeIdentity{NodeRecord: &NodeRecord{id: 2}, index: 22},
		&nodeIdentity{NodeRecord: &NodeRecord{id: 5}, index: 55},
	}

	sort.Sort(nodes)

	Expect(nodes).To(Equal(
		nodeIdentities{
			&nodeIdentity{NodeRecord: &NodeRecord{id: 1}, index: 11},
			&nodeIdentity{NodeRecord: &NodeRecord{id: 2}, index: 22},
			&nodeIdentity{NodeRecord: &NodeRecord{id: 3}, index: 33},
			&nodeIdentity{NodeRecord: &NodeRecord{id: 5}, index: 55},
		},
	))
}

func TestFilterByID(t *testing.T) {
	RegisterTestingT(t)

	nodes := []*NodeRecord{
		&NodeRecord{id: 1, address: "a", role: CollectorRole},
		&NodeRecord{id: 5, address: "b", role: ConsensusRole},
		&NodeRecord{id: 3, address: "c", role: CollectorRole},
	}
	tbl := NewInMemoryIdentityTable(nodes)

	tfiltered, err := tbl.FilterByID([]uint{5, 3})
	expected := []*NodeRecord{
		&NodeRecord{id: 5, address: "b", role: ConsensusRole},
		&NodeRecord{id: 3, address: "c", role: CollectorRole},
	}
	Ω(err).ShouldNot(HaveOccurred())
	Ω(tfiltered).Should(Equal(NewInMemoryIdentityTable(expected)))

	tfiltered, err = tbl.FilterByID([]uint{5, 20, 3})
	Ω(err).Should(HaveOccurred())
	Ω(tfiltered).Should(Equal(NewInMemoryIdentityTable(expected)))

	tfiltered, err = tbl.FilterByID([]uint{20})
	Ω(err).Should(HaveOccurred())
	Ω(tfiltered).Should(Equal(NewInMemoryIdentityTable([]*NodeRecord{})))
}

func TestFilterByAddress(t *testing.T) {
	RegisterTestingT(t)

	nodes := []*NodeRecord{
		&NodeRecord{id: 1, address: "a", role: CollectorRole},
		&NodeRecord{id: 5, address: "b", role: ConsensusRole},
		&NodeRecord{id: 3, address: "c", role: CollectorRole},
	}
	tbl := NewInMemoryIdentityTable(nodes)
	tfiltered, err := tbl.FilterByAddress([]string{"b", "a"})

	expected := []*NodeRecord{
		&NodeRecord{id: 1, address: "a", role: CollectorRole},
		&NodeRecord{id: 5, address: "b", role: ConsensusRole},
	}
	Ω(err).ShouldNot(HaveOccurred())
	Ω(tfiltered).Should(Equal(NewInMemoryIdentityTable(expected)))

	tfiltered, err = tbl.FilterByAddress([]string{"b", "unknown", "a"})
	Ω(err).Should(HaveOccurred())
	Ω(tfiltered).Should(Equal(NewInMemoryIdentityTable(expected)))

	tfiltered, err = tbl.FilterByAddress([]string{"unknown"})
	Ω(err).Should(HaveOccurred())
	Ω(tfiltered).Should(Equal(NewInMemoryIdentityTable([]*NodeRecord{})))
}

func TestFilterByRole(t *testing.T) {
	RegisterTestingT(t)

	nodes := []*NodeRecord{
		&NodeRecord{id: 1, address: "a", role: CollectorRole},
		&NodeRecord{id: 5, address: "b", role: ConsensusRole},
		&NodeRecord{id: 3, address: "c", role: CollectorRole},
	}
	tbl := NewInMemoryIdentityTable(nodes)
	tfiltered, err := tbl.FilterByRole(ConsensusRole)

	expected := []*NodeRecord{
		&NodeRecord{id: 5, address: "b", role: ConsensusRole},
	}
	Ω(err).ShouldNot(HaveOccurred())
	Ω(tfiltered).Should(Equal(NewInMemoryIdentityTable(expected)))

	tfiltered, err = tbl.FilterByRole(VerifierRole)
	Ω(err).Should(HaveOccurred())
	Ω(tfiltered).Should(Equal(NewInMemoryIdentityTable([]*NodeRecord{})))
}

func TestFilterByIndex(t *testing.T) {
	RegisterTestingT(t)

	nodes := []*NodeRecord{
		&NodeRecord{id: 1, address: "a", role: CollectorRole}, // assigned index 0 after sorting by ID
		&NodeRecord{id: 5, address: "b", role: ConsensusRole}, // assigned index 2 after sorting by ID
		&NodeRecord{id: 3, address: "c", role: CollectorRole}, // assigned index 1 after sorting by ID
	}
	tbl := NewInMemoryIdentityTable(nodes)
	tfiltered, err := tbl.FilterByIndex([]uint{2, 1})

	expected := []*NodeRecord{
		&NodeRecord{id: 5, address: "b", role: ConsensusRole},
		&NodeRecord{id: 3, address: "c", role: CollectorRole},
	}
	Ω(err).ShouldNot(HaveOccurred())
	Ω(tfiltered).Should(Equal(NewInMemoryIdentityTable(expected)))

	tfiltered, err = tbl.FilterByIndex([]uint{3, 2, 1})
	Ω(err).Should(HaveOccurred())
	Ω(tfiltered).Should(Equal(NewInMemoryIdentityTable(expected)))

	tfiltered, err = tbl.FilterByIndex([]uint{3})
	Ω(err).Should(HaveOccurred())
	Ω(tfiltered).Should(Equal(NewInMemoryIdentityTable([]*NodeRecord{})))
}
