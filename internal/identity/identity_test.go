package identity_test

import (
	"github.com/dapperlabs/bamboo-node/internal/identity"
	"sort"
	"testing"

	. "github.com/onsi/gomega"
)

func TestNewInMemoryIdentityTable(t *testing.T) {
	RegisterTestingT(t)

	nodes := identity.NodeRecords{
		&identity.NodeRecord{ID: 1},
		&identity.NodeRecord{ID: 3},
		&identity.NodeRecord{ID: 2},
		&identity.NodeRecord{ID: 5},
	}

	sort.Sort(nodes)

	Expect(nodes).To(Equal(
		identity.NodeRecords{
			&identity.NodeRecord{ID: 1},
			&identity.NodeRecord{ID: 2},
			&identity.NodeRecord{ID: 3},
			&identity.NodeRecord{ID: 5},
		},
	))
}