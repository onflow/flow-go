package packer

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff/packer"
	"github.com/onflow/flow-go/model/flow"
)

func EncodeSignerIdentifiersToIndices(fullIdentities []flow.Identifier, signerIDs flow.IdentifierList) ([]byte, error) {
	signersLookup := signerIDs.Lookup()

	indices := make([]int, 0, len(fullIdentities))
	for i, member := range fullIdentities {
		if _, ok := signersLookup[member]; ok {
			indices = append(indices, i)
			delete(signersLookup, member)
		}
	}

	if len(signersLookup) > 0 {
		return nil, fmt.Errorf("unknown signers %v", signersLookup)
	}

	signerIndices := packer.EncodeSignerIndices(indices, len(fullIdentities))
	return signerIndices, nil
}

func DecodeSignerIdentifiersFromIndices(fullIdentities []flow.Identifier, signerIndices []byte) ([]flow.Identifier, error) {
	panic("to be implemented")
}
