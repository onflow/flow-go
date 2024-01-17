package environment_test

import (
	"testing"
	"testing/quick"

	"github.com/onflow/cadence/runtime/common"
	testMock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/environment/mock"
	"github.com/onflow/flow-go/fvm/tracing"
)

func newDummyAccountKeyReader(
	t *testing.T,
	keyCount uint64,
) environment.AccountKeyReader {
	tracer := tracing.NewTracerSpan()
	meter := mock.NewMeter(t)
	meter.On("MeterComputation", testMock.Anything, testMock.Anything).Return(nil)
	accounts := &FakeAccounts{keyCount: keyCount}
	return environment.NewAccountKeyReader(tracer, meter, accounts)
}

func bytesToAddress(bytes ...uint8) common.Address {
	return common.MustBytesToAddress(bytes)
}

func TestKeyConversionValidAlgorithms(t *testing.T) {
	t.Parallel()

	t.Run("invalid hash algo", func(t *testing.T) {
		t.Parallel()

		accountKey := FakePublicKey{}.toAccountPublicKey()
		accountKey.HashAlgo = hash.UnknownHashingAlgorithm

		rtKey, err := environment.FlowToRuntimeAccountKey(accountKey)
		require.Error(t, err)
		require.Nil(t, rtKey)
	})

	t.Run("invalid sign algo", func(t *testing.T) {
		t.Parallel()

		accountKey := FakePublicKey{}.toAccountPublicKey()
		accountKey.SignAlgo = crypto.UnknownSigningAlgorithm

		rtKey, err := environment.FlowToRuntimeAccountKey(accountKey)
		require.Error(t, err)
		require.Nil(t, rtKey)
	})

	t.Run("valid key", func(t *testing.T) {
		t.Parallel()

		accountKey := FakePublicKey{}.toAccountPublicKey()

		rtKey, err := environment.FlowToRuntimeAccountKey(accountKey)
		require.NoError(t, err)
		require.NotNil(t, rtKey)

		require.Equal(t, accountKey.PublicKey.Encode(), rtKey.PublicKey.PublicKey)
	})
}
func TestAccountKeyReader_get_valid_key(t *testing.T) {
	t.Parallel()
	address := bytesToAddress(1, 2, 3, 4)

	res, err := newDummyAccountKeyReader(t, 10).GetAccountKey(address, 0)
	require.NoError(t, err)

	expected, err := environment.FlowToRuntimeAccountKey(
		FakePublicKey{}.toAccountPublicKey(),
	)

	require.NoError(t, err)

	require.Equal(t, expected, res)
}

func TestAccountKeyReader_get_out_of_range(t *testing.T) {
	t.Parallel()
	address := bytesToAddress(1, 2, 3, 4)

	res, err := newDummyAccountKeyReader(t, 0).GetAccountKey(address, 1000)
	// GetAccountKey should distinguish between an invalid index, and issues like failing to fetch a key from storage
	require.Nil(t, err)
	require.Nil(t, res)
}

func TestAccountKeyReader_get_key_count(t *testing.T) {
	t.Parallel()
	address := bytesToAddress(1, 2, 3, 4)

	identity := func(n uint64) (uint64, error) { return n, nil }
	prop := func(n uint64) (uint64, error) {
		return newDummyAccountKeyReader(t, n).AccountKeysCount(address)
	}

	if err := quick.CheckEqual(identity, prop, nil); err != nil {
		t.Error(err)
	}
}
