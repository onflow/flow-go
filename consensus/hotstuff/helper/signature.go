package helper

import (
	"github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func MakeWeightedSignatureAggregator(sigWeight uint64) *mocks.WeightedSignatureAggregator {
	stakingSigAggtor := &mocks.WeightedSignatureAggregator{}
	var totalWeight uint64
	stakingSigAggtor.On("TrustedAdd", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		totalWeight += sigWeight
	}).Return(func(signerID flow.Identifier, sig crypto.Signature) uint64 {
		return totalWeight
	}, func(signerID flow.Identifier, sig crypto.Signature) error {
		return nil
	}).Maybe()
	stakingSigAggtor.On("TotalWeight").Return(func() uint64 {
		return totalWeight
	}).Maybe()
	stakingSigAggtor.On("Aggregate").Return(unittest.IdentifierListFixture(5),
		unittest.RandomBytes(48), nil).Maybe()
	return stakingSigAggtor
}

func MakeRandomBeaconReconstructor(minRequiredShares int) *mocks.RandomBeaconReconstructor {
	rbRector := &mocks.RandomBeaconReconstructor{}
	rbSharesTotal := 0
	rbRector.On("TrustedAdd", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		rbSharesTotal++
	}).Return(func(signerID flow.Identifier, sig crypto.Signature) bool {
		return rbSharesTotal >= minRequiredShares
	}, func(signerID flow.Identifier, sig crypto.Signature) error {
		return nil
	}).Maybe()
	rbRector.On("EnoughShares").Return(func() bool {
		return rbSharesTotal >= minRequiredShares
	}).Maybe()
	rbRector.On("Reconstruct").Return(unittest.SignatureFixture(), nil).Maybe()
	return rbRector
}
