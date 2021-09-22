package signature_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/module/metrics"
	modmocks "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/signature"
	storage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestGetThresholdSignerWithNilPrivateKey tests that without a random beacon private key
// the signer always returns a invalid BLS signature
func TestGetThresholdSignerWithNilPrivateKey(t *testing.T) {

	// epoch counter
	epoch := uint64(1)

	// build epoch lookup mock
	epochLookup := new(modmocks.EpochLookup)
	epochLookup.On("EpochForViewWithFallback", mock.Anything).Return(epoch, nil)

	unittest.RunWithTypedBadgerDB(t, storage.InitSecret, func(db *badger.DB) {

		dkgKeys, err := storage.NewDKGKeys(metrics.NewNoopCollector(), db)
		assert.NoError(t, err)
		signerStore := signature.NewEpochAwareSignerStore(epochLookup, dkgKeys)

		signer, err := signerStore.GetThresholdSigner(uint64(1))
		require.NoError(t, err)

		signed, err := signer.Sign([]byte{})
		require.NoError(t, err)
		assert.Equal(t, signed, crypto.BLSInvalidSignature())
	})
}
