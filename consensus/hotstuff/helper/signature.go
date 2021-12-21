package helper

import (
	"github.com/stretchr/testify/mock"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func MakeWeightedSignatureAggregator(sigWeight uint64) *mocks.WeightedSignatureAggregator {
	stakingSigAggtor := &mocks.WeightedSignatureAggregator{}
	totalWeight := atomic.NewUint64(0)
	stakingSigAggtor.On("TrustedAdd", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		totalWeight.Add(sigWeight)
	}).Return(func(signerID flow.Identifier, sig crypto.Signature) uint64 {
		return totalWeight.Load()
	}, func(signerID flow.Identifier, sig crypto.Signature) error {
		return nil
	}).Maybe()
	stakingSigAggtor.On("TotalWeight").Return(func() uint64 {
		return totalWeight.Load()
	}).Maybe()
	stakingSigAggtor.On("Aggregate").Return(unittest.IdentifierListFixture(5),
		unittest.RandomBytes(48), nil).Maybe()
	return stakingSigAggtor
}

func MakeRandomBeaconReconstructor(minRequiredShares int) *mocks.RandomBeaconReconstructor {
	rbRector := &mocks.RandomBeaconReconstructor{}
	rbSharesTotal := atomic.NewUint64(0)
	rbRector.On("TrustedAdd", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		rbSharesTotal.Inc()
	}).Return(func(signerID flow.Identifier, sig crypto.Signature) bool {
		return rbSharesTotal.Load() >= uint64(minRequiredShares)
	}, func(signerID flow.Identifier, sig crypto.Signature) error {
		return nil
	}).Maybe()
	rbRector.On("EnoughShares").Return(func() bool {
		return rbSharesTotal.Load() >= uint64(minRequiredShares)
	}).Maybe()
	rbRector.On("Reconstruct").Return(unittest.SignatureFixture(), nil).Maybe()
	return rbRector
}
