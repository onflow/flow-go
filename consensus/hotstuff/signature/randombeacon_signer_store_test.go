package signature

import (
	"errors"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/crypto"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/module"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/storage"
	mockstorage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type BeaconKeyStore struct {
	suite.Suite
	epochLookup *mockmodule.EpochLookup
	beaconKeys  *mockstorage.SafeBeaconKeys
	store       *EpochAwareRandomBeaconKeyStore
}

func TestBeaconKeyStore(t *testing.T) {
	suite.Run(t, new(BeaconKeyStore))
}

func (suite *BeaconKeyStore) SetupTest() {
	suite.epochLookup = mockmodule.NewEpochLookup(suite.T())
	suite.beaconKeys = mockstorage.NewSafeBeaconKeys(suite.T())
	suite.store = NewEpochAwareRandomBeaconKeyStore(suite.epochLookup, suite.beaconKeys)
}

// TestHappyPath tests the happy path, where the epoch is known and there is a safe beacon key available.
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

// Test_EpochLookup_UnknownEpochError tests that when EpochLookup returns
// model.ErrViewForUnknownEpoch it is propagated to the caller of ByView.
func (suite *BeaconKeyStore) Test_EpochLookup_ViewForUnknownEpoch() {
	view := rand.Uint64()
	suite.epochLookup.On("EpochForViewWithFallback", view).Return(uint64(0), model.ErrViewForUnknownEpoch)

	key, err := suite.store.ByView(view)
	require.ErrorIs(suite.T(), err, model.ErrViewForUnknownEpoch)
	assert.Nil(suite.T(), key)
}

// Test_EpochLookup_UnexpectedError tests that an exception from EpochLookup is
// propagated to the caller of ByView.
func (suite *BeaconKeyStore) Test_EpochLookup_UnexpectedError() {
	view := rand.Uint64()
	exception := errors.New("unexpected error")
	suite.epochLookup.On("EpochForViewWithFallback", view).Return(uint64(0), exception)

	key, err := suite.store.ByView(view)
	require.ErrorIs(suite.T(), err, exception)
	assert.Nil(suite.T(), key)
}

// Test_BeaconKeys_Unsafe tests that if SafeBeaconKeys reports an unsafe key,
// ByView returns that no beacon key is available.
func (suite *BeaconKeyStore) Test_BeaconKeys_Unsafe() {
	view := rand.Uint64()
	epoch := rand.Uint64()
	suite.epochLookup.On("EpochForViewWithFallback", view).Return(epoch, nil)
	suite.beaconKeys.On("RetrieveMyBeaconPrivateKey", epoch).Return(nil, false, nil)

	key, err := suite.store.ByView(view)
	require.ErrorIs(suite.T(), err, module.ErrNoBeaconKeyForEpoch)
	assert.Nil(suite.T(), key)
}

// Test_BeaconKeys_NotFound tests that if SafeBeaconKeys returns storage.ErrNotFound,
// ByView returns that no beacon key is available.
func (suite *BeaconKeyStore) Test_BeaconKeys_NotFound() {
	view := rand.Uint64()
	epoch := rand.Uint64()
	suite.epochLookup.On("EpochForViewWithFallback", view).Return(epoch, nil)
	suite.beaconKeys.On("RetrieveMyBeaconPrivateKey", epoch).Return(nil, false, storage.ErrNotFound)

	key, err := suite.store.ByView(view)
	require.ErrorIs(suite.T(), err, module.ErrNoBeaconKeyForEpoch)
	assert.Nil(suite.T(), key)
}

// Test_BeaconKeys_NotFound tests that if SafeBeaconKeys returns storage.ErrNotFound,
// ByView initially returns that no beacon key is available. But if SafeBeaconKeys
// later returns a safe key, ByView should thereafter return the beacon key.
// In other words, NotFound results should not be cached.
func (suite *BeaconKeyStore) Test_BeaconKeys_NotFoundThenAvailable() {
	view := rand.Uint64()
	epoch := rand.Uint64()
	suite.epochLookup.On("EpochForViewWithFallback", view).Return(epoch, nil)

	var retKey crypto.PrivateKey
	var retSafe bool
	var retErr error
	suite.beaconKeys.On("RetrieveMyBeaconPrivateKey", epoch).
		Return(
			func(_ uint64) crypto.PrivateKey { return retKey },
			func(_ uint64) bool { return retSafe },
			func(_ uint64) error { return retErr },
		)

	// 1 - return storage.ErrNotFound
	retKey = nil
	retSafe = false
	retErr = storage.ErrNotFound

	key, err := suite.store.ByView(view)
	require.ErrorIs(suite.T(), err, module.ErrNoBeaconKeyForEpoch)
	assert.Nil(suite.T(), key)

	// 2 - return a safe beacon key
	retKey = unittest.RandomBeaconPriv().PrivateKey
	retSafe = true
	retErr = nil

	key, err = suite.store.ByView(view)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), retKey, key)
}
