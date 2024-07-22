package kvstore_test

import (
	"github.com/onflow/flow-go/model/flow"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	"github.com/onflow/flow-go/state/protocol/protocol_state/kvstore"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestEncodeDecode tests encoding and decoding all supported model versions.
//   - VersionedEncode should return the correct version
//   - instances should be equal after encoding, then decoding
func TestEncodeDecode(t *testing.T) {
	t.Run("v0", func(t *testing.T) {
		model := &kvstore.Modelv0{
			UpgradableModel: kvstore.UpgradableModel{
				VersionUpgrade: &protocol.ViewBasedActivator[uint64]{
					Data:           13,
					ActivationView: 1000,
				},
			},
			EpochStateID: unittest.IdentifierFixture(),
		}

		version, encoded, err := model.VersionedEncode()
		require.NoError(t, err)
		assert.Equal(t, uint64(0), version)

		decoded, err := kvstore.VersionedDecode(version, encoded)
		require.NoError(t, err)
		assert.Equal(t, model, decoded)
	})

	t.Run("v1", func(t *testing.T) {
		model := &kvstore.Modelv1{}

		version, encoded, err := model.VersionedEncode()
		require.NoError(t, err)
		assert.Equal(t, uint64(1), version)

		decoded, err := kvstore.VersionedDecode(version, encoded)
		require.NoError(t, err)
		assert.Equal(t, model, decoded)
	})
}

// TestKVStoreAPI tests that all supported model versions satisfy the public interfaces.
//   - should be able to read/write supported keys
//   - should return the appropriate sentinel for unsupported keys
func TestKVStoreAPI(t *testing.T) {
	t.Run("v0", func(t *testing.T) {
		model := &kvstore.Modelv0{}

		// v0
		assertModelIsUpgradable(t, model)

		version := model.GetProtocolStateVersion()
		assert.Equal(t, uint64(0), version)
	})

	t.Run("v1", func(t *testing.T) {
		model := &kvstore.Modelv1{}

		// v0
		assertModelIsUpgradable(t, model)

		version := model.GetProtocolStateVersion()
		assert.Equal(t, uint64(1), version)
	})
}

// TestKVStoreAPI_Replicate tests that replication logic of KV store correctly works. All versions need to be support this.
// There are a few invariants that needs to be met:
// - if model M is replicated and the requested version is equal to M.Version then an exact copy needs to be returned.
// - if model M is replicated and the requested version is lower than M.Version then an error has to be returned.
// - if model M is replicated and the requested version is greater than M.Version then behavior depends on concrete model.
// If replication from version v to v' is not supported a sentinel error should be returned, otherwise component needs to return
// a new model with version which is equal to the requested version.
func TestKVStoreAPI_Replicate(t *testing.T) {
	t.Run("v0", func(t *testing.T) {
		model := &kvstore.Modelv0{
			UpgradableModel: kvstore.UpgradableModel{
				VersionUpgrade: &protocol.ViewBasedActivator[uint64]{
					Data:           13,
					ActivationView: 1000,
				},
			},
		}
		cpy, err := model.Replicate(model.GetProtocolStateVersion())
		require.NoError(t, err)
		require.True(t, reflect.DeepEqual(model, cpy)) // expect the same model
		require.Equal(t, cpy.ID(), model.ID())

		model.VersionUpgrade.ActivationView++ // change
		require.False(t, reflect.DeepEqual(model, cpy), "expect to have a deep copy")
	})
	t.Run("v0->v1", func(t *testing.T) {
		model := &kvstore.Modelv0{
			UpgradableModel: kvstore.UpgradableModel{
				VersionUpgrade: &protocol.ViewBasedActivator[uint64]{
					Data:           13,
					ActivationView: 1000,
				},
			},
		}
		newVersion, err := model.Replicate(1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), newVersion.GetProtocolStateVersion())
		require.NotEqual(t, newVersion.ID(), model.ID(), "two models with the same data but different version must have different ID")
		_, ok := newVersion.(*kvstore.Modelv1)
		require.True(t, ok, "expected Modelv1")
		require.Equal(t, newVersion.GetVersionUpgrade(), model.GetVersionUpgrade())
	})
	t.Run("v0-invalid-upgrade", func(t *testing.T) {
		model := &kvstore.Modelv0{}
		newVersion, err := model.Replicate(model.GetProtocolStateVersion() + 10)
		require.ErrorIs(t, err, kvstore.ErrIncompatibleVersionChange)
		require.Nil(t, newVersion)
	})
	t.Run("v1", func(t *testing.T) {
		model := &kvstore.Modelv1{
			Modelv0: kvstore.Modelv0{
				UpgradableModel: kvstore.UpgradableModel{
					VersionUpgrade: &protocol.ViewBasedActivator[uint64]{
						Data:           13,
						ActivationView: 1000,
					},
				},
				EpochStateID: unittest.IdentifierFixture(),
			},
		}
		cpy, err := model.Replicate(model.GetProtocolStateVersion())
		require.NoError(t, err)
		require.True(t, reflect.DeepEqual(model, cpy))

		model.VersionUpgrade.ActivationView++ // change
		require.False(t, reflect.DeepEqual(model, cpy))
	})
	t.Run("v1-invalid-upgrade", func(t *testing.T) {
		model := &kvstore.Modelv1{}

		for _, version := range []uint64{
			model.GetProtocolStateVersion() - 1,
			model.GetProtocolStateVersion() + 1,
			model.GetProtocolStateVersion() + 10,
		} {
			newVersion, err := model.Replicate(version)
			require.ErrorIs(t, err, kvstore.ErrIncompatibleVersionChange)
			require.Nil(t, newVersion)
		}
	})
}

// assertModelIsUpgradable tests that the model satisfies the version upgrade interface.
//   - should be able to set and get the upgrade version
//   - setting nil version upgrade should work
//
// This has to be tested for every model version since version upgrade should be supported by all models.
func assertModelIsUpgradable(t *testing.T, api protocol_state.KVStoreMutator) {
	oldVersion := api.GetProtocolStateVersion()
	activationView := uint64(1000)
	expected := &protocol.ViewBasedActivator[uint64]{
		Data:           oldVersion + 1,
		ActivationView: activationView,
	}

	// check if setting version upgrade works
	api.SetVersionUpgrade(expected)
	actual := api.GetVersionUpgrade()
	assert.Equal(t, expected, actual, "version upgrade should be set")

	// check if setting nil version upgrade works
	api.SetVersionUpgrade(nil)
	assert.Nil(t, api.GetVersionUpgrade(), "version upgrade should be nil")
}

// TestNewDefaultKVStore tests that the default KV store is created correctly.
func TestNewDefaultKVStore(t *testing.T) {
	t.Run("happy-path", func(t *testing.T) {
		safetyParams, err := protocol.DefaultEpochSafetyParams(flow.Localnet)
		require.NoError(t, err)
		epochStateID := unittest.IdentifierFixture()
		store, err := kvstore.NewDefaultKVStore(safetyParams.FinalizationSafetyThreshold, safetyParams.EpochExtensionViewCount, epochStateID)
		require.NoError(t, err)
		require.Equal(t, store.GetEpochStateID(), epochStateID)
		require.Equal(t, store.GetEpochCommitSafetyThreshold(), safetyParams.FinalizationSafetyThreshold)
		require.Equal(t, store.GetEpochExtensionViewCount(), safetyParams.EpochExtensionViewCount)
		require.GreaterOrEqual(t, store.GetEpochExtensionViewCount(), 2*safetyParams.FinalizationSafetyThreshold,
			"extension view count should be at least 2*FinalizationSafetyThreshold")
	})
	t.Run("invalid-epoch-extension-view-count", func(t *testing.T) {
		safetyParams, err := protocol.DefaultEpochSafetyParams(flow.Localnet)
		require.NoError(t, err)
		epochStateID := unittest.IdentifierFixture()
		// invalid epoch extension view count, it has to be at least 2*FinalizationSafetyThreshold
		store, err := kvstore.NewDefaultKVStore(safetyParams.FinalizationSafetyThreshold, safetyParams.FinalizationSafetyThreshold, epochStateID)
		require.Error(t, err)
		require.Nil(t, store)
	})
}

// TestKVStoreMutator_SetEpochExtensionViewCount tests that setter performs an input validation and doesn't allow setting
// a value which is lower than 2*FinalizationSafetyThreshold.
func TestKVStoreMutator_SetEpochExtensionViewCount(t *testing.T) {
	safetyParams, err := protocol.DefaultEpochSafetyParams(flow.Localnet)
	require.NoError(t, err)
	epochStateID := unittest.IdentifierFixture()

	t.Run("happy-path", func(t *testing.T) {
		store, err := kvstore.NewDefaultKVStore(safetyParams.FinalizationSafetyThreshold, safetyParams.EpochExtensionViewCount, epochStateID)
		require.NoError(t, err)
		mutator, err := store.Replicate(store.GetProtocolStateVersion())
		require.NoError(t, err)

		newValue := safetyParams.FinalizationSafetyThreshold*2 + 1
		require.NotEqual(t, mutator.GetEpochExtensionViewCount(), newValue)
		err = mutator.SetEpochExtensionViewCount(newValue)
		require.NoError(t, err)
		require.Equal(t, mutator.GetEpochExtensionViewCount(), newValue)
	})
	t.Run("invalid-value", func(t *testing.T) {
		store, err := kvstore.NewDefaultKVStore(safetyParams.FinalizationSafetyThreshold, safetyParams.EpochExtensionViewCount, epochStateID)
		require.NoError(t, err)
		mutator, err := store.Replicate(store.GetProtocolStateVersion())
		require.NoError(t, err)

		oldValue := mutator.GetEpochExtensionViewCount()
		newValue := safetyParams.FinalizationSafetyThreshold*2 - 1
		require.NotEqual(t, oldValue, newValue)
		err = mutator.SetEpochExtensionViewCount(newValue)
		require.ErrorIs(t, err, kvstore.ErrInvalidValue)
		require.Equal(t, mutator.GetEpochExtensionViewCount(), oldValue, "value should be unchanged")
	})

}
