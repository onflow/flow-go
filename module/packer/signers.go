package packer

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

func FilterByIndices(identities flow.IdentityList, indices []int) (flow.IdentityList, error) {
	list := make([]*flow.Identity, 0, len(identities))
	for _, index := range indices {
		if index > len(identities)-1 {
			return nil, fmt.Errorf("signer index %v is out of range %v", index, len(identities)-1)
		}
		list = append(list, identities[index])
	}
	return list, nil
}
