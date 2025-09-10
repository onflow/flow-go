package operation_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestInstanceParams_InsertRetrieve verifies that InstanceParams can be
// correctly stored and retrieved from the database as a single encoded
// structure.
func TestInstanceParams_InsertRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		enc, err := operation.NewVersionedInstanceParams(
			operation.DefaultInstanceParamsVersion,
			unittest.IdentifierFixture(),
			unittest.IdentifierFixture(),
			unittest.IdentifierFixture(),
		)
		require.NoError(t, err)

		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertInstanceParams(rw, *enc)
		})
		require.NoError(t, err)

		var actual operation.VersionedInstanceParams
		err = operation.RetrieveInstanceParams(db.Reader(), &actual)
		require.NoError(t, err)

		require.Equal(t, enc.Version, actual.Version)
		require.Equal(t, enc.InstanceParams, actual.InstanceParams.(operation.InstanceParamsV0))
	})
}

// TestNewVersionedInstanceParams verifies that NewVersionedInstanceParams
// correctly constructs instance params for supported versions and
// returns an error for unsupported versions.
// Test cases:
// 1) If version is 0, it constructs InstanceParamsV0 with the provided data.
// 2) If version is unsupported, it returns an error.
func TestNewVersionedInstanceParams(t *testing.T) {
	finalizedRootID := unittest.IdentifierFixture()
	sealedRootID := unittest.IdentifierFixture()
	sporkRootID := unittest.IdentifierFixture()

	t.Run("valid version 0", func(t *testing.T) {
		versioned, err := operation.NewVersionedInstanceParams(0, finalizedRootID, sealedRootID, sporkRootID)
		require.NoError(t, err)
		require.Equal(t, uint64(0), versioned.Version)

		params, ok := versioned.InstanceParams.(operation.InstanceParamsV0)
		require.True(t, ok)
		require.Equal(t, finalizedRootID, params.FinalizedRootID)
		require.Equal(t, sealedRootID, params.SealedRootID)
		require.Equal(t, sporkRootID, params.SporkRootBlockID)
	})

	t.Run("unsupported version", func(t *testing.T) {
		versioned, err := operation.NewVersionedInstanceParams(99, finalizedRootID, sealedRootID, sporkRootID)
		require.Nil(t, versioned)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported instance params version")
	})
}

// TestVersionedInstanceParams_UnmarshalMsgpack verifies that
// UnmarshalMsgpack correctly decodes VersionedInstanceParams bytes.
// Test cases:
// 1) Valid version 0 bytes decode correctly into VersionedInstanceParams.
// 2) Unsupported version bytes, it returns an error.
func TestVersionedInstanceParams_UnmarshalMsgpack(t *testing.T) {
	expected, err := operation.NewVersionedInstanceParams(0, unittest.IdentifierFixture(), unittest.IdentifierFixture(), unittest.IdentifierFixture())
	require.NoError(t, err)

	b, err := msgpack.Marshal(expected)
	require.NoError(t, err)

	t.Run("unmarshal valid version 0", func(t *testing.T) {
		var decoded operation.VersionedInstanceParams
		err := decoded.UnmarshalMsgpack(b)
		require.NoError(t, err)
		require.Equal(t, expected.Version, decoded.Version)
		require.Equal(t, expected.InstanceParams, decoded.InstanceParams.(operation.InstanceParamsV0))
	})

	t.Run("unmarshal unsupported version", func(t *testing.T) {
		type invalid struct {
			Version        uint64
			InstanceParams interface{}
		}
		unsupported, err := msgpack.Marshal(&invalid{
			Version:        99,
			InstanceParams: struct{}{},
		})
		require.NoError(t, err)

		var decoded operation.VersionedInstanceParams
		err = decoded.UnmarshalMsgpack(unsupported)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported instance params version")
	})
}
