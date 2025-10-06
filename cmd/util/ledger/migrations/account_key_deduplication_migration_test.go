package migrations

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/fxamacker/circlehash"
	"github.com/onflow/cadence/common"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestAccountPublicKeyDeduplicator(t *testing.T) {
	pk1 := newAccountPublicKey(t, 1000)
	pkb1, err := encodeStoredPublicKeyFromAccountPublicKey(pk1)
	require.NoError(t, err)

	pk2 := newAccountPublicKey(t, 1000)
	pkb2, err := encodeStoredPublicKeyFromAccountPublicKey(pk2)
	require.NoError(t, err)

	pk3 := newAccountPublicKey(t, 1000)
	pkb3, err := encodeStoredPublicKeyFromAccountPublicKey(pk3)
	require.NoError(t, err)

	pk4 := newAccountPublicKey(t, 1000)
	pkb4, err := encodeStoredPublicKeyFromAccountPublicKey(pk4)
	require.NoError(t, err)

	pk5 := newAccountPublicKey(t, 1000)
	pkb5, err := encodeStoredPublicKeyFromAccountPublicKey(pk5)
	require.NoError(t, err)

	testcases := []struct {
		name              string
		keys              []flow.AccountPublicKey
		encodedUniqueKeys [][]byte
		mappings          []uint32
		hasDeduplicate    bool
	}{
		{
			name: "empty",
		},
		{
			name:              "1 key",
			keys:              []flow.AccountPublicKey{pk1},
			encodedUniqueKeys: [][]byte{pkb1},
			mappings:          []uint32{0},
		},
		{
			name:              "2 unique key",
			keys:              []flow.AccountPublicKey{pk1, pk2},
			encodedUniqueKeys: [][]byte{pkb1, pkb2},
			mappings:          []uint32{0, 1},
		},
		{
			name:              "2 duplicate key",
			keys:              []flow.AccountPublicKey{pk1, pk1},
			encodedUniqueKeys: [][]byte{pkb1},
			mappings:          []uint32{0, 0},
			hasDeduplicate:    true,
		},
		{
			name:              "5 keys with 1 unique keys",
			keys:              []flow.AccountPublicKey{pk1, pk1, pk1, pk1, pk1},
			encodedUniqueKeys: [][]byte{pkb1},
			mappings:          []uint32{0, 0, 0, 0, 0},
			hasDeduplicate:    true,
		},
		{
			name:              "5 keys with 4 unique keys",
			keys:              []flow.AccountPublicKey{pk1, pk2, pk1, pk3, pk4},
			encodedUniqueKeys: [][]byte{pkb1, pkb2, pkb3, pkb4},
			mappings:          []uint32{0, 1, 0, 2, 3},
			hasDeduplicate:    true,
		},
		{
			name:              "5 keys with 5 unique keys",
			keys:              []flow.AccountPublicKey{pk1, pk2, pk3, pk4, pk5},
			encodedUniqueKeys: [][]byte{pkb1, pkb2, pkb3, pkb4, pkb5},
			mappings:          []uint32{0, 1, 2, 3, 4},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			deduplicator := newAccountPublicKeyDeduplicator("a", uint32(len(tc.keys)))

			for i, pk := range tc.keys {
				err = deduplicator.addAccountPublicKey(uint32(i), pk)
				require.NoError(t, err)
			}

			require.Equal(t, tc.hasDeduplicate, deduplicator.hasDuplicateKey())

			keys := deduplicator.uniqueKeys()
			require.ElementsMatch(t, tc.encodedUniqueKeys, keys)

			mapping := deduplicator.keyIndexMapping()
			require.ElementsMatch(t, tc.mappings, mapping)
		})
	}
}

func TestDigestForLastNEncodedPublicKeys(t *testing.T) {
	var owner [8]byte
	_, _ = rand.Read(owner[:])
	seed := binary.BigEndian.Uint64(owner[:])

	type pkData struct {
		pk     flow.AccountPublicKey
		pkb    []byte
		digest uint64
	}

	pks := make([]pkData, 3)

	for {
		digestsMap := make(map[uint64]struct{})

		for i := range len(pks) {
			pk := newAccountPublicKey(t, 1000)

			pkb, err := encodeStoredPublicKeyFromAccountPublicKey(pk)
			require.NoError(t, err)

			digest := circlehash.Hash64(pkb, seed)

			pks[i] = pkData{pk, pkb, digest}
			digestsMap[digest] = struct{}{}
		}

		if len(digestsMap) == len(pks) {
			break
		}
	}

	testcases := []struct {
		name               string
		encodedPublicKeys  [][]byte
		hashFunc           func(b []byte, seed uint64) uint64
		n                  int
		expectedStartIndex int
		expectedDigests    []uint64
	}{
		{
			name: "empty",
			n:    2,
		},
		{
			name:               "1 key, min 2 keys",
			n:                  2,
			encodedPublicKeys:  [][]byte{pks[0].pkb},
			expectedStartIndex: 0,
			expectedDigests:    []uint64{pks[0].digest},
		},
		{
			name:               "2 key, min 2 keys",
			n:                  2,
			encodedPublicKeys:  [][]byte{pks[0].pkb, pks[1].pkb},
			expectedStartIndex: 0,
			expectedDigests:    []uint64{pks[0].digest, pks[1].digest},
		},
		{
			name:               "3 key, min 2 keys",
			n:                  2,
			encodedPublicKeys:  [][]byte{pks[0].pkb, pks[1].pkb, pks[2].pkb},
			expectedStartIndex: 1,
			expectedDigests:    []uint64{pks[1].digest, pks[2].digest},
		},
		{
			name: "3 key, min 2 keys, collision",
			n:    2,
			hashFunc: func([]byte, uint64) uint64 {
				return 0
			},
			encodedPublicKeys:  [][]byte{pks[0].pkb, pks[1].pkb, pks[2].pkb},
			expectedStartIndex: 1,
			expectedDigests:    []uint64{0, dummyDigest},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			startIndex, digests := generateLastNPublicKeyDigests(
				zerolog.Nop(),
				string(owner[:]),
				tc.encodedPublicKeys,
				tc.n,
				tc.hashFunc,
			)
			require.Equal(t, tc.expectedStartIndex, startIndex)
			require.ElementsMatch(t, tc.expectedDigests, digests)
		})
	}
}

func TestMigration(t *testing.T) {
	const accountStatusMinSize = 29

	t.Run("no account public key", func(t *testing.T) {
		var owner [8]byte
		_, _ = rand.Read(owner[:])

		encodedAccountStatusV3 := newEncodedAccountStatusV3(0)

		accountRegisters := registers.NewAccountRegisters(string(owner[:]))
		err := accountRegisters.Set(string(owner[:]), flow.AccountStatusKey, encodedAccountStatusV3)
		require.NoError(t, err)

		deduplicated, err := migrateAndDeduplicateAccountPublicKeys(
			zerolog.Nop(),
			accountRegisters,
		)
		require.NoError(t, err)
		require.False(t, deduplicated)

		// Register after migration:
		// - "a.s"
		require.Equal(t, 1, accountRegisters.Count())

		// Test "a.s" register
		encodedAccountStatusV4, err := accountRegisters.Get(string(owner[:]), flow.AccountStatusKey)
		require.NoError(t, err)
		require.Equal(t, len(encodedAccountStatusV3), len(encodedAccountStatusV4))
		require.Equal(t, byte(0x40), encodedAccountStatusV4[0])
		require.Equal(t, encodedAccountStatusV3[1:], encodedAccountStatusV4[1:])

		err = ValidateAccountPublicKeyV4(common.Address(owner), accountRegisters)
		require.NoError(t, err)
	})

	t.Run("1 account public key without sequence number", func(t *testing.T) {
		var owner [8]byte
		_, _ = rand.Read(owner[:])

		pk1 := newAccountPublicKey(t, 1000)
		encodedPk1, err := flow.EncodeAccountPublicKey(pk1)
		require.NoError(t, err)

		encodedAccountStatusV3 := newEncodedAccountStatusV3(1)

		accountRegisters := registers.NewAccountRegisters(string(owner[:]))
		err = accountRegisters.Set(string(owner[:]), flow.AccountStatusKey, encodedAccountStatusV3)
		require.NoError(t, err)
		err = accountRegisters.Set(string(owner[:]), legacyAccountPublicKey0RegisterKey, encodedPk1)
		require.NoError(t, err)

		deduplicated, err := migrateAndDeduplicateAccountPublicKeys(
			zerolog.Nop(),
			accountRegisters,
		)
		require.NoError(t, err)
		require.False(t, deduplicated)

		// Registers after migration:
		// - "a.s"
		// - "apk_0"
		require.Equal(t, 2, accountRegisters.Count())

		// Test "a.s" register
		encodedAccountStatusV4, err := accountRegisters.Get(string(owner[:]), flow.AccountStatusKey)
		require.NoError(t, err)
		require.Equal(t, len(encodedAccountStatusV3), len(encodedAccountStatusV4))
		require.Equal(t, byte(0x40), encodedAccountStatusV4[0])
		require.Equal(t, encodedAccountStatusV3[1:], encodedAccountStatusV4[1:])

		// Test "apk_0" register
		encodedAccountPublicKey0, err := accountRegisters.Get(string(owner[:]), flow.AccountPublicKey0RegisterKey)
		require.NoError(t, err)
		require.Equal(t, encodedPk1, encodedAccountPublicKey0)

		err = ValidateAccountPublicKeyV4(common.Address(owner), accountRegisters)
		require.NoError(t, err)
	})

	t.Run("1 account public key with sequence number", func(t *testing.T) {
		var owner [8]byte
		_, _ = rand.Read(owner[:])

		pk1 := newAccountPublicKey(t, 1000)
		pk1.SeqNumber = 1
		encodedPk1, err := flow.EncodeAccountPublicKey(pk1)
		require.NoError(t, err)

		encodedAccountStatusV3 := newEncodedAccountStatusV3(1)

		accountRegisters := registers.NewAccountRegisters(string(owner[:]))
		err = accountRegisters.Set(string(owner[:]), flow.AccountStatusKey, encodedAccountStatusV3)
		require.NoError(t, err)
		err = accountRegisters.Set(string(owner[:]), legacyAccountPublicKey0RegisterKey, encodedPk1)
		require.NoError(t, err)

		deduplicated, err := migrateAndDeduplicateAccountPublicKeys(
			zerolog.Nop(),
			accountRegisters,
		)
		require.NoError(t, err)
		require.False(t, deduplicated)

		// Registers after migration:
		// - "a.s"
		// - "apk_0"
		require.Equal(t, 2, accountRegisters.Count())

		// Test "a.s" register
		encodedAccountStatusV4, err := accountRegisters.Get(string(owner[:]), flow.AccountStatusKey)
		require.NoError(t, err)
		require.Equal(t, len(encodedAccountStatusV3), len(encodedAccountStatusV4))
		require.Equal(t, byte(0x40), encodedAccountStatusV4[0])
		require.Equal(t, encodedAccountStatusV3[1:], encodedAccountStatusV4[1:])

		// Test "apk_0" register
		encodedAccountPublicKey0, err := accountRegisters.Get(string(owner[:]), flow.AccountPublicKey0RegisterKey)
		require.NoError(t, err)
		require.Equal(t, encodedPk1, encodedAccountPublicKey0)

		err = ValidateAccountPublicKeyV4(common.Address(owner), accountRegisters)
		require.NoError(t, err)
	})

	t.Run("2 unique account public key without sequence number", func(t *testing.T) {
		var owner [8]byte
		_, _ = rand.Read(owner[:])
		seed := binary.BigEndian.Uint64(owner[:])

		pk1 := newAccountPublicKey(t, 1000)
		encodedPk1, err := flow.EncodeAccountPublicKey(pk1)
		require.NoError(t, err)

		encodedSpk1, err := encodeStoredPublicKeyFromAccountPublicKey(pk1)
		require.NoError(t, err)

		digest1 := circlehash.Hash64(encodedSpk1, seed)

		pk2 := newAccountPublicKey(t, 1000)
		encodedPk2, err := flow.EncodeAccountPublicKey(pk2)
		require.NoError(t, err)

		encodedSpk2, err := encodeStoredPublicKeyFromAccountPublicKey(pk2)
		require.NoError(t, err)

		digest2 := circlehash.Hash64(encodedSpk2, seed)

		encodedAccountStatusV3 := newEncodedAccountStatusV3(2)

		accountRegisters := registers.NewAccountRegisters(string(owner[:]))
		err = accountRegisters.Set(string(owner[:]), flow.AccountStatusKey, encodedAccountStatusV3)
		require.NoError(t, err)
		err = accountRegisters.Set(string(owner[:]), legacyAccountPublicKey0RegisterKey, encodedPk1)
		require.NoError(t, err)
		err = accountRegisters.Set(string(owner[:]), fmt.Sprintf(legacyAccountPublicKeyRegisterKeyPattern, 1), encodedPk2)
		require.NoError(t, err)

		deduplicated, err := migrateAndDeduplicateAccountPublicKeys(
			zerolog.Nop(),
			accountRegisters,
		)
		require.NoError(t, err)
		require.False(t, deduplicated)

		// Registers after migration:
		// - "a.s"
		// - "apk_0"
		// - "pk_b0"
		require.Equal(t, 3, accountRegisters.Count())

		// Test "a.s" register
		encodedAccountStatusV4, err := accountRegisters.Get(string(owner[:]), flow.AccountStatusKey)
		require.NoError(t, err)
		require.True(t, len(encodedAccountStatusV3) < len(encodedAccountStatusV4))
		require.Equal(t, byte(0x40), encodedAccountStatusV4[0])
		require.Equal(t, encodedAccountStatusV3[1:], encodedAccountStatusV4[1:len(encodedAccountStatusV3)])

		_, weightAndRevokedStatus, startKeyIndexForDigests, digests, startKeyIndexForMapping, accountPublicKeyMappings, err := decodeAccountStatusV4(encodedAccountStatusV4)
		require.NoError(t, err)
		require.ElementsMatch(t, []accountPublicKeyWeightAndRevokedStatus{{1000, false}}, weightAndRevokedStatus)
		require.Equal(t, uint32(0), startKeyIndexForDigests)
		require.ElementsMatch(t, []uint64{digest1, digest2}, digests)
		require.Equal(t, uint32(0), startKeyIndexForMapping)
		require.Nil(t, accountPublicKeyMappings)

		// Test "apk_0" register
		encodedAccountPublicKey0, err := accountRegisters.Get(string(owner[:]), flow.AccountPublicKey0RegisterKey)
		require.NoError(t, err)
		require.Equal(t, encodedPk1, encodedAccountPublicKey0)

		// Test "pk_b0" register
		encodedBatchPublicKey0, err := accountRegisters.Get(string(owner[:]), fmt.Sprintf(flow.BatchPublicKeyRegisterKeyPattern, 0))
		require.NoError(t, err)

		encodedPks, err := decodeBatchPublicKey(encodedBatchPublicKey0)
		require.NoError(t, err)
		require.ElementsMatch(t, [][]byte{{}, encodedSpk2}, encodedPks)

		err = ValidateAccountPublicKeyV4(common.Address(owner), accountRegisters)
		require.NoError(t, err)
	})

	t.Run("2 unique account public key with sequence number", func(t *testing.T) {
		var owner [8]byte
		_, _ = rand.Read(owner[:])
		seed := binary.BigEndian.Uint64(owner[:])

		pk1 := newAccountPublicKey(t, 1000)
		pk1.SeqNumber = 1
		encodedPk1, err := flow.EncodeAccountPublicKey(pk1)
		require.NoError(t, err)

		encodedSpk1, err := encodeStoredPublicKeyFromAccountPublicKey(pk1)
		require.NoError(t, err)

		digest1 := circlehash.Hash64(encodedSpk1, seed)

		pk2 := newAccountPublicKey(t, 1000)
		pk2.SeqNumber = 2
		encodedPk2, err := flow.EncodeAccountPublicKey(pk2)
		require.NoError(t, err)

		encodedSpk2, err := encodeStoredPublicKeyFromAccountPublicKey(pk2)
		require.NoError(t, err)

		digest2 := circlehash.Hash64(encodedSpk2, seed)

		encodedAccountStatusV3 := newEncodedAccountStatusV3(2)

		accountRegisters := registers.NewAccountRegisters(string(owner[:]))
		err = accountRegisters.Set(string(owner[:]), flow.AccountStatusKey, encodedAccountStatusV3)
		require.NoError(t, err)
		err = accountRegisters.Set(string(owner[:]), legacyAccountPublicKey0RegisterKey, encodedPk1)
		require.NoError(t, err)
		err = accountRegisters.Set(string(owner[:]), fmt.Sprintf(legacyAccountPublicKeyRegisterKeyPattern, 1), encodedPk2)
		require.NoError(t, err)

		deduplicated, err := migrateAndDeduplicateAccountPublicKeys(
			zerolog.Nop(),
			accountRegisters,
		)
		require.NoError(t, err)
		require.False(t, deduplicated)

		// Registers after migration:
		// - "a.s"
		// - "apk_0"
		// - "pk_b0"
		// - "sn_1"
		require.Equal(t, 4, accountRegisters.Count())

		// Test "a.s" register
		encodedAccountStatusV4, err := accountRegisters.Get(string(owner[:]), flow.AccountStatusKey)
		require.NoError(t, err)
		require.True(t, len(encodedAccountStatusV3) < len(encodedAccountStatusV4))
		require.Equal(t, byte(0x40), encodedAccountStatusV4[0])
		require.Equal(t, encodedAccountStatusV3[1:], encodedAccountStatusV4[1:len(encodedAccountStatusV3)])

		_, weightAndRevokedStatus, startKeyIndexForDigests, digests, startKeyIndexForMapping, accountPublicKeyMappings, err := decodeAccountStatusV4(encodedAccountStatusV4)
		require.NoError(t, err)
		require.ElementsMatch(t, []accountPublicKeyWeightAndRevokedStatus{{1000, false}}, weightAndRevokedStatus)
		require.Equal(t, uint32(0), startKeyIndexForDigests)
		require.ElementsMatch(t, []uint64{digest1, digest2}, digests)
		require.Equal(t, uint32(0), startKeyIndexForMapping)
		require.Nil(t, accountPublicKeyMappings)

		// Test "apk_0" register
		encodedAccountPublicKey0, err := accountRegisters.Get(string(owner[:]), flow.AccountPublicKey0RegisterKey)
		require.NoError(t, err)
		require.Equal(t, encodedPk1, encodedAccountPublicKey0)

		// Test "pk_b0" register
		encodedBatchPublicKey0, err := accountRegisters.Get(string(owner[:]), fmt.Sprintf(flow.BatchPublicKeyRegisterKeyPattern, 0))
		require.NoError(t, err)

		encodedPks, err := decodeBatchPublicKey(encodedBatchPublicKey0)
		require.NoError(t, err)
		require.ElementsMatch(t, [][]byte{{}, encodedSpk2}, encodedPks)

		// Test "sn_0" register
		encodedSequenceNumber, err := accountRegisters.Get(string(owner[:]), fmt.Sprintf(flow.SequenceNumberRegisterKeyPattern, 1))
		require.NoError(t, err)

		seqNum, err := flow.DecodeSequenceNumber(encodedSequenceNumber)
		require.NoError(t, err)
		require.Equal(t, uint64(2), seqNum)

		err = ValidateAccountPublicKeyV4(common.Address(owner), accountRegisters)
		require.NoError(t, err)
	})

	t.Run("2 account public key (1 unique key) without sequence number", func(t *testing.T) {
		var owner [8]byte
		_, _ = rand.Read(owner[:])
		seed := binary.BigEndian.Uint64(owner[:])

		pk1 := newAccountPublicKey(t, 1000)
		encodedPk1, err := flow.EncodeAccountPublicKey(pk1)
		require.NoError(t, err)

		encodedSpk1, err := encodeStoredPublicKeyFromAccountPublicKey(pk1)
		require.NoError(t, err)

		digest1 := circlehash.Hash64(encodedSpk1, seed)

		encodedAccountStatusV3 := newEncodedAccountStatusV3(2)

		accountRegisters := registers.NewAccountRegisters(string(owner[:]))
		err = accountRegisters.Set(string(owner[:]), flow.AccountStatusKey, encodedAccountStatusV3)
		require.NoError(t, err)
		err = accountRegisters.Set(string(owner[:]), legacyAccountPublicKey0RegisterKey, encodedPk1)
		require.NoError(t, err)
		err = accountRegisters.Set(string(owner[:]), fmt.Sprintf(legacyAccountPublicKeyRegisterKeyPattern, 1), encodedPk1)
		require.NoError(t, err)

		deduplicated, err := migrateAndDeduplicateAccountPublicKeys(
			zerolog.Nop(),
			accountRegisters,
		)
		require.NoError(t, err)
		require.True(t, deduplicated)

		// Registers after migration:
		// - "a.s"
		// - "apk_0"
		require.Equal(t, 2, accountRegisters.Count())

		// Test "a.s" register
		encodedAccountStatusV4, err := accountRegisters.Get(string(owner[:]), flow.AccountStatusKey)
		require.NoError(t, err)
		require.True(t, len(encodedAccountStatusV3) < len(encodedAccountStatusV4))
		require.Equal(t, byte(0x41), encodedAccountStatusV4[0])
		require.Equal(t, encodedAccountStatusV3[1:], encodedAccountStatusV4[1:len(encodedAccountStatusV3)])

		_, weightAndRevokedStatus, startKeyIndexForDigests, digests, startKeyIndexForMapping, accountPublicKeyMappings, err := decodeAccountStatusV4(encodedAccountStatusV4)
		require.NoError(t, err)
		require.ElementsMatch(t, []accountPublicKeyWeightAndRevokedStatus{{1000, false}}, weightAndRevokedStatus)
		require.Equal(t, uint32(0), startKeyIndexForDigests)
		require.ElementsMatch(t, []uint64{digest1}, digests)
		require.Equal(t, uint32(1), startKeyIndexForMapping)
		require.ElementsMatch(t, []uint32{0}, accountPublicKeyMappings)

		// Test "apk_0" register
		encodedAccountPublicKey0, err := accountRegisters.Get(string(owner[:]), flow.AccountPublicKey0RegisterKey)
		require.NoError(t, err)
		require.Equal(t, encodedPk1, encodedAccountPublicKey0)

		err = ValidateAccountPublicKeyV4(common.Address(owner), accountRegisters)
		require.NoError(t, err)
	})

	t.Run("2 account public key (1 unique key) with sequence number", func(t *testing.T) {
		var owner [8]byte
		_, _ = rand.Read(owner[:])
		seed := binary.BigEndian.Uint64(owner[:])

		pk1 := newAccountPublicKey(t, 1000)
		pk1.SeqNumber = 1
		encodedPk1, err := flow.EncodeAccountPublicKey(pk1)
		require.NoError(t, err)

		encodedSpk1, err := encodeStoredPublicKeyFromAccountPublicKey(pk1)
		require.NoError(t, err)

		digest1 := circlehash.Hash64(encodedSpk1, seed)

		encodedAccountStatusV3 := newEncodedAccountStatusV3(2)

		accountRegisters := registers.NewAccountRegisters(string(owner[:]))
		err = accountRegisters.Set(string(owner[:]), flow.AccountStatusKey, encodedAccountStatusV3)
		require.NoError(t, err)
		err = accountRegisters.Set(string(owner[:]), legacyAccountPublicKey0RegisterKey, encodedPk1)
		require.NoError(t, err)
		err = accountRegisters.Set(string(owner[:]), fmt.Sprintf(legacyAccountPublicKeyRegisterKeyPattern, 1), encodedPk1)
		require.NoError(t, err)

		deduplicated, err := migrateAndDeduplicateAccountPublicKeys(
			zerolog.Nop(),
			accountRegisters,
		)
		require.NoError(t, err)
		require.True(t, deduplicated)

		// Registers after migration:
		// - "a.s"
		// - "apk_0"
		// - "sn_1"
		require.Equal(t, 3, accountRegisters.Count())

		// Test "a.s" register
		encodedAccountStatusV4, err := accountRegisters.Get(string(owner[:]), flow.AccountStatusKey)
		require.NoError(t, err)
		require.True(t, len(encodedAccountStatusV3) < len(encodedAccountStatusV4))
		require.Equal(t, byte(0x41), encodedAccountStatusV4[0])
		require.Equal(t, encodedAccountStatusV3[1:], encodedAccountStatusV4[1:len(encodedAccountStatusV3)])

		_, weightAndRevokedStatus, startKeyIndexForDigests, digests, startKeyIndexForMapping, accountPublicKeyMappings, err := decodeAccountStatusV4(encodedAccountStatusV4)
		require.NoError(t, err)
		require.ElementsMatch(t, []accountPublicKeyWeightAndRevokedStatus{{1000, false}}, weightAndRevokedStatus)
		require.Equal(t, uint32(0), startKeyIndexForDigests)
		require.ElementsMatch(t, []uint64{digest1}, digests)
		require.Equal(t, uint32(1), startKeyIndexForMapping)
		require.ElementsMatch(t, []uint32{0}, accountPublicKeyMappings)

		// Test "apk_0" register
		encodedAccountPublicKey0, err := accountRegisters.Get(string(owner[:]), flow.AccountPublicKey0RegisterKey)
		require.NoError(t, err)
		require.Equal(t, encodedPk1, encodedAccountPublicKey0)

		// Test "sn_0" register
		encodedSequenceNumber, err := accountRegisters.Get(string(owner[:]), fmt.Sprintf(flow.SequenceNumberRegisterKeyPattern, 1))
		require.NoError(t, err)

		seqNum, err := flow.DecodeSequenceNumber(encodedSequenceNumber)
		require.NoError(t, err)
		require.Equal(t, uint64(1), seqNum)

		err = ValidateAccountPublicKeyV4(common.Address(owner), accountRegisters)
		require.NoError(t, err)
	})
}

func newAccountPublicKey(t *testing.T, weight int) flow.AccountPublicKey {
	privateKey, err := unittest.AccountKeyDefaultFixture()
	require.NoError(t, err)

	return privateKey.PublicKey(weight)
}

func newEncodedAccountStatusV3(accountPublicKeyCount uint32) []byte {
	b := []byte{
		0,                      // initial empty flags
		0, 0, 0, 0, 0, 0, 0, 0, // init value for storage used
		0, 0, 0, 0, 0, 0, 0, 1, // init value for storage index
		0, 0, 0, 0, // init value for public key counts
		0, 0, 0, 0, 0, 0, 0, 0, // init value for address id counter
	}

	var encodedAccountPublicKeyCount [4]byte
	binary.BigEndian.PutUint32(encodedAccountPublicKeyCount[:], accountPublicKeyCount)

	copy(b[17:], encodedAccountPublicKeyCount[:])

	return b
}
