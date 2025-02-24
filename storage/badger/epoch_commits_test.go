package badger_test

import (
	"errors"
	"io"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/crypto"
	"github.com/onflow/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	badgerstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestEpochCommitStoreAndRetrieve tests that a commit can be stored, retrieved and attempted to be stored again without an error
func TestEpochCommitStoreAndRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := badgerstorage.NewEpochCommits(metrics, db)

		// attempt to get a invalid commit
		_, err := store.ByID(unittest.IdentifierFixture())
		assert.True(t, errors.Is(err, storage.ErrNotFound))

		// store a commit in db
		expected := unittest.EpochCommitFixture()
		err = transaction.Update(db, func(tx *transaction.Tx) error {
			return store.StoreTx(expected)(tx)
		})
		require.NoError(t, err)

		// retrieve the commit by ID
		actual, err := store.ByID(expected.ID())
		require.NoError(t, err)
		assert.Equal(t, expected, actual)

		// test storing same epoch commit
		err = transaction.Update(db, func(tx *transaction.Tx) error {
			return store.StoreTx(expected)(tx)
		})
		require.NoError(t, err)
	})
}

// epochCommitV0 is a version of [flow.EpochCommit] without the [flow.DKGIndexMap] field.
// This exact structure was used prior to Protocol State Version 2, and we would like to ensure that new version of [flow.EpochCommit]
// is backward compatible with this structure.
// It is used only in tests.
// TODO(EFM, #6794): Remove this once we complete the network upgrade
type epochCommitV0 struct {
	// Counter is the epoch counter of the epoch being committed
	Counter uint64
	// ClusterQCs is an ordered list of root quorum certificates, one per cluster.
	// EpochCommit.ClustersQCs[i] is the QC for EpochSetup.Assignments[i]
	ClusterQCs []flow.ClusterQCVoteData
	// DKGGroupKey is the group public key produced by the DKG associated with this epoch.
	// It is used to verify Random Beacon signatures for the epoch with counter, Counter.
	DKGGroupKey crypto.PublicKey
	// DKGParticipantKeys is a list of public keys, one per DKG participant, ordered by Random Beacon index.
	// This list is the output of the DKG associated with this epoch.
	// It is used to verify Random Beacon signatures for the epoch with counter, Counter.
	// CAUTION: This list may include keys for nodes which do not exist in the consensus committee
	//          and may NOT include keys for all nodes in the consensus committee.
	DKGParticipantKeys []crypto.PublicKey
}

func (commit *epochCommitV0) ID() flow.Identifier {
	return flow.MakeID(commit)
}

func (commit *epochCommitV0) EncodeRLP(w io.Writer) error {
	rlpEncodable := struct {
		Counter            uint64
		ClusterQCs         []flow.ClusterQCVoteData
		DKGGroupKey        []byte
		DKGParticipantKeys [][]byte
	}{
		Counter:            commit.Counter,
		ClusterQCs:         commit.ClusterQCs,
		DKGGroupKey:        commit.DKGGroupKey.Encode(),
		DKGParticipantKeys: make([][]byte, 0, len(commit.DKGParticipantKeys)),
	}
	for _, key := range commit.DKGParticipantKeys {
		rlpEncodable.DKGParticipantKeys = append(rlpEncodable.DKGParticipantKeys, key.Encode())
	}

	return rlp.Encode(w, rlpEncodable)
}

// encodableCommit represents encoding of epochCommitV0, it is used for serialization purposes and is used only in tests.
// TODO(EFM, #6794): Remove this once we complete the network upgrade
type encodableCommit struct {
	Counter            uint64
	ClusterQCs         []flow.ClusterQCVoteData
	DKGGroupKey        encodable.RandomBeaconPubKey
	DKGParticipantKeys []encodable.RandomBeaconPubKey
}

func encodableFromCommit(commit *epochCommitV0) encodableCommit {
	encKeys := make([]encodable.RandomBeaconPubKey, 0, len(commit.DKGParticipantKeys))
	for _, key := range commit.DKGParticipantKeys {
		encKeys = append(encKeys, encodable.RandomBeaconPubKey{PublicKey: key})
	}
	return encodableCommit{
		Counter:            commit.Counter,
		ClusterQCs:         commit.ClusterQCs,
		DKGGroupKey:        encodable.RandomBeaconPubKey{PublicKey: commit.DKGGroupKey},
		DKGParticipantKeys: encKeys,
	}
}

func commitFromEncodable(enc encodableCommit) epochCommitV0 {
	dkgKeys := make([]crypto.PublicKey, 0, len(enc.DKGParticipantKeys))
	for _, key := range enc.DKGParticipantKeys {
		dkgKeys = append(dkgKeys, key.PublicKey)
	}
	return epochCommitV0{
		Counter:            enc.Counter,
		ClusterQCs:         enc.ClusterQCs,
		DKGGroupKey:        enc.DKGGroupKey.PublicKey,
		DKGParticipantKeys: dkgKeys,
	}
}

func (commit *epochCommitV0) MarshalMsgpack() ([]byte, error) {
	return msgpack.Marshal(encodableFromCommit(commit))
}

func (commit *epochCommitV0) UnmarshalMsgpack(b []byte) error {
	var enc encodableCommit
	err := msgpack.Unmarshal(b, &enc)
	if err != nil {
		return err
	}
	*commit = commitFromEncodable(enc)
	return nil
}

// TestStoreV0AndDecodeV1 tests that an [flow.EpochCommit] without [flow.DKGIndexMap](v0) field can be stored and
// later retrieved as a [flow.EpochCommit](v1) without any errors or data loss.
// This test verifies that the [flow.EpochCommit] is backward compatible with respect to the [flow.DKGIndexMap] field.
// TODO(EFM, #6794): Remove this once we complete the network upgrade
func TestStoreV0AndDecodeV1(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		v1 := unittest.EpochCommitFixture()
		v0 := &epochCommitV0{
			Counter:            v1.Counter,
			ClusterQCs:         v1.ClusterQCs,
			DKGGroupKey:        v1.DKGGroupKey,
			DKGParticipantKeys: v1.DKGParticipantKeys,
		}
		require.Equal(t, v0.ID(), v1.ID())

		err := transaction.Update(db, func(tx *transaction.Tx) error {
			return operation.InsertEpochCommitV0(v0.ID(), v0)(tx.DBTxn)
		})
		require.NoError(t, err)

		var actual flow.EpochCommit
		err = transaction.View(db, func(tx *transaction.Tx) error {
			return operation.RetrieveEpochCommit(v0.ID(), &actual)(tx.DBTxn)
		})
		require.NoError(t, err)
		require.Equal(t, v1, &actual)
		require.Equal(t, v0.ID(), actual.ID())
		require.Equal(t, v1, &actual)
	})

}
