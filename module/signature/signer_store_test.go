package signature_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	modmocks "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/signature"
	storage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestGetThresholdSigner tests that without a valid random beacon private key
// the signer always returns an invalid BLS signature
func TestGetThresholdSigner(t *testing.T) {

	epochCounter := uint64(1)
	// build epoch lookup mock
	epochLookup := new(modmocks.EpochLookup)
	epochLookup.On("EpochForViewWithFallback", mock.Anything).Return(epochCounter, nil)

	t.Run("without key - failed dkg", func(t *testing.T) {
		unittest.RunWithTypedBadgerDB(t, storage.InitSecret, func(db *badger.DB) {
			metrics := metrics.NewNoopCollector()
			dkgState, err := storage.NewDKGState(metrics, db)
			require.NoError(t, err)
			dkgKeys, err := storage.NewSafeBeaconPrivateKeys(dkgState)
			require.NoError(t, err)

			err = dkgState.SetDKGEndState(epochCounter, flow.DKGEndStateDKGFailure)
			require.NoError(t, err)

			signerStore := signature.NewEpochAwareSignerStore(epochLookup, dkgKeys)

			signer, err := signerStore.GetThresholdSigner(uint64(1))
			require.NoError(t, err)

			signed, err := signer.Sign([]byte{})
			require.NoError(t, err)
			assert.Equal(t, signed, crypto.BLSInvalidSignature())
		})
	})

	t.Run("key not marked valid", func(t *testing.T) {
		unittest.RunWithTypedBadgerDB(t, storage.InitSecret, func(db *badger.DB) {
			metrics := metrics.NewNoopCollector()
			dkgState, err := storage.NewDKGState(metrics, db)
			require.NoError(t, err)
			dkgKeys, err := storage.NewSafeBeaconPrivateKeys(dkgState)
			require.NoError(t, err)

			// store a key, mark the DKG as failed
			err = dkgState.InsertMyBeaconPrivateKey(epochCounter, unittest.RandomBeaconPriv().PrivateKey)
			require.NoError(t, err)
			err = dkgState.SetDKGEndState(epochCounter, flow.DKGEndStateInconsistentKey)
			require.NoError(t, err)

			signerStore := signature.NewEpochAwareSignerStore(epochLookup, dkgKeys)

			signer, err := signerStore.GetThresholdSigner(uint64(1))
			require.NoError(t, err)

			signed, err := signer.Sign([]byte{})
			require.NoError(t, err)
			assert.Equal(t, signed, crypto.BLSInvalidSignature())
		})
	})
}
