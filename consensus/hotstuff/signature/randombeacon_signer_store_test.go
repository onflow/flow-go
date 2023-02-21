package signature

import (
	"errors"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/module"
	mockmodule "github.com/onflow/flow-go/module/mock"
	mockstorage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type BeaconKeyStore struct {
	suite.Suite
	epochLookup *mockmodule.EpochLookup
	beaconKeys  *mockstorage.SafeBeaconKeys
	store       *EpochAwareRandomBeaconKeyStore

	view  uint64
	epoch uint64
}

func TestBeaconKeyStore(t *testing.T) {
	suite.Run(t, new(BeaconKeyStore))
}

func (suite *BeaconKeyStore) SetupTest() {
	suite.epochLookup = mockmodule.NewEpochLookup(suite.T())
	suite.beaconKeys = mockstorage.NewSafeBeaconKeys(suite.T())
	suite.store = NewEpochAwareRandomBeaconKeyStore(suite.epochLookup, suite.beaconKeys)
}

func (suite *BeaconKeyStore) TestHappyPath() {
	view := rand.Uint64()
	epoch := rand.Uint64()
	expectedKey := unittest.KeyFixture(crypto.BLSBLS12381)
	suite.epochLookup.On("EpochForViewWithFallback", view).Return(epoch, nil)
	suite.beaconKeys.On("RetrieveMyBeaconPrivateKey", epoch).Return(expectedKey, true, nil)

	key, err := suite.store.ByView(view)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedKey, key)
}

func (suite *BeaconKeyStore) Test_EpochLookup_UnknownEpochError() {
	view := rand.Uint64()
	suite.epochLookup.On("EpochForViewWithFallback", view).Return(0, model.ErrViewForUnknownEpoch)

	key, err := suite.store.ByView(view)
	require.ErrorIs(suite.T(), err, model.ErrViewForUnknownEpoch)
	assert.Nil(suite.T(), key)
}

func (suite *BeaconKeyStore) Test_EpochLookup_UnexpectedError() {
	view := rand.Uint64()
	exception := errors.New("unexpected error")
	suite.epochLookup.On("EpochForViewWithFallback", view).Return(0, exception)

	key, err := suite.store.ByView(view)
	require.ErrorIs(suite.T(), err, exception)
	assert.Nil(suite.T(), key)
}

func (suite *BeaconKeyStore) Test_BeaconKeys_Unsafe() {
	view := rand.Uint64()
	epoch := rand.Uint64()
	suite.epochLookup.On("EpochForViewWithFallback", view).Return(epoch, nil)
	suite.beaconKeys.On("RetrieveMyBeaconPrivateKey", epoch).Return(nil, false, nil)

	key, err := suite.store.ByView(view)
	require.ErrorIs(suite.T(), err, module.DKGFailError)
}

// ErrVIewForUnknownEpoch
// unexpected error
//
// key, nil
// nil, unsafe, nil
// nil, unsafe, ErrNotFound
