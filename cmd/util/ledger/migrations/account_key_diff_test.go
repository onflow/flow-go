package migrations

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/common"
	"github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	accountkeymetadata "github.com/onflow/flow-go/fvm/environment/account-key-metadata"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

func TestAccountPublicKeyDiff(t *testing.T) {
	address := flow.BytesToAddress([]byte{0x01})
	chainID := flow.Testnet

	t.Run("0 key", func(t *testing.T) {
		accountStatusV3Bytes := environment.NewAccountStatus().ToBytes()
		accountStatusV3Bytes[0] = 0
		registersV3, err := registers.NewByAccountFromPayloads([]*ledger.Payload{
			newPayload(address, flow.AccountStatusKey, accountStatusV3Bytes),
		})
		require.NoError(t, err)

		accountStatusV4Bytes := environment.NewAccountStatus().ToBytes()
		registersV4, err := registers.NewByAccountFromPayloads([]*ledger.Payload{
			newPayload(address, flow.AccountStatusKey, accountStatusV4Bytes),
		})
		require.NoError(t, err)

		reportWriter := newMemoryReportWriter()

		diffReporter := NewAccountKeyDiffReporter(common.Address(address), chainID, reportWriter)
		diffReporter.DiffKeys(registersV3, registersV4)

		// No diff
		require.Equal(t, 0, len(reportWriter.data))
	})

	t.Run("1 key", func(t *testing.T) {
		key0 := newAccountPublicKey(t, 1000)
		storedKey0 := accountPublicKeyToStoredKey(key0)

		encodedKey0, err := flow.EncodeAccountPublicKey(key0)
		require.NoError(t, err)

		encodedStoredKey0, err := flow.EncodeStoredPublicKey(storedKey0)
		require.NoError(t, err)

		accountStatusV3 := environment.NewAccountStatus()
		accountStatusV3.SetAccountPublicKeyCount(1)
		accountStatusV3Bytes := accountStatusV3.ToBytes()
		accountStatusV3Bytes[0] = 0

		registersV3, err := registers.NewByAccountFromPayloads([]*ledger.Payload{
			newPayload(address, flow.AccountStatusKey, accountStatusV3Bytes),
			newPayload(address, legacyAccountPublicKey0RegisterKey, encodedKey0),
		})
		require.NoError(t, err)

		accountStatusV4 := environment.NewAccountStatus()
		// Append key 0
		_, _, err = accountStatusV4.AppendAccountPublicKeyMetadata(
			key0.Revoked,
			uint16(key0.Weight),
			encodedStoredKey0,
			func(b []byte) uint64 {
				return accountkeymetadata.GetPublicKeyDigest(address, b)
			},
			func(i uint32) ([]byte, error) {
				return nil, fmt.Errorf("don't expect getStoredKey called, got %d", i)
			},
		)
		require.NoError(t, err)

		registersV4, err := registers.NewByAccountFromPayloads([]*ledger.Payload{
			newPayload(address, flow.AccountStatusKey, accountStatusV4.ToBytes()),
			newPayload(address, "apk_0", encodedKey0),
		})
		require.NoError(t, err)

		reportWriter := newMemoryReportWriter()

		diffReporter := NewAccountKeyDiffReporter(common.Address(address), chainID, reportWriter)
		diffReporter.DiffKeys(registersV3, registersV4)

		// No diff
		require.Equal(t, 0, len(reportWriter.data))
	})

	t.Run("2 keys", func(t *testing.T) {
		key0 := newAccountPublicKey(t, 1000)
		storedKey0 := accountPublicKeyToStoredKey(key0)

		encodedKey0, err := flow.EncodeAccountPublicKey(key0)
		require.NoError(t, err)

		encodedStoredKey0, err := flow.EncodeStoredPublicKey(storedKey0)
		require.NoError(t, err)

		key1 := newAccountPublicKey(t, 1)
		storedKey1 := accountPublicKeyToStoredKey(key1)

		encodedKey1, err := flow.EncodeAccountPublicKey(key1)
		require.NoError(t, err)

		encodedStoredKey1, err := flow.EncodeStoredPublicKey(storedKey1)
		require.NoError(t, err)

		accountStatusV3 := environment.NewAccountStatus()
		accountStatusV3.SetAccountPublicKeyCount(2)
		accountStatusV3Bytes := accountStatusV3.ToBytes()
		accountStatusV3Bytes[0] = 0

		registersV3, err := registers.NewByAccountFromPayloads([]*ledger.Payload{
			newPayload(address, flow.AccountStatusKey, accountStatusV3Bytes),
			newPayload(address, legacyAccountPublicKey0RegisterKey, encodedKey0),
			newPayload(address, fmt.Sprintf(legacyAccountPublicKeyRegisterKeyPattern, 1), encodedKey1),
		})
		require.NoError(t, err)

		accountStatusV4 := environment.NewAccountStatus()
		// Append key 0
		_, _, err = accountStatusV4.AppendAccountPublicKeyMetadata(
			key0.Revoked,
			uint16(key0.Weight),
			encodedStoredKey0,
			func(b []byte) uint64 {
				return accountkeymetadata.GetPublicKeyDigest(address, b)
			},
			func(i uint32) ([]byte, error) {
				return nil, fmt.Errorf("don't expect getStoredKey called, got %d", i)
			},
		)
		require.NoError(t, err)
		// Append key 1
		_, _, err = accountStatusV4.AppendAccountPublicKeyMetadata(
			key1.Revoked,
			uint16(key1.Weight),
			encodedStoredKey1,
			func(b []byte) uint64 {
				return accountkeymetadata.GetPublicKeyDigest(address, b)
			},
			func(i uint32) ([]byte, error) {
				if i == 0 {
					return encodedStoredKey0, nil
				}
				return nil, fmt.Errorf("expect getStoredKey for key index 0, got %d", i)
			},
		)
		require.NoError(t, err)

		registersV4, err := registers.NewByAccountFromPayloads([]*ledger.Payload{
			newPayload(address, flow.AccountStatusKey, accountStatusV4.ToBytes()),
			newPayload(address, "apk_0", encodedKey0),
			newPayload(address, "pk_b0", newBatchPublicKey(t, []*flow.StoredPublicKey{nil, &storedKey1})),
		})
		require.NoError(t, err)

		reportWriter := newMemoryReportWriter()

		diffReporter := NewAccountKeyDiffReporter(common.Address(address), chainID, reportWriter)
		diffReporter.DiffKeys(registersV3, registersV4)

		// No diff
		require.Equal(t, 0, len(reportWriter.data))
	})

	t.Run("2 keys, diff for public key", func(t *testing.T) {
		key0 := newAccountPublicKey(t, 1000)
		storedKey0 := accountPublicKeyToStoredKey(key0)

		encodedKey0, err := flow.EncodeAccountPublicKey(key0)
		require.NoError(t, err)

		encodedStoredKey0, err := flow.EncodeStoredPublicKey(storedKey0)
		require.NoError(t, err)

		key1a := newAccountPublicKey(t, 1)

		encodedKey1a, err := flow.EncodeAccountPublicKey(key1a)
		require.NoError(t, err)

		key1b := newAccountPublicKey(t, 1)
		storedKey1b := accountPublicKeyToStoredKey(key1b)

		encodedStoredKey1b, err := flow.EncodeStoredPublicKey(storedKey1b)
		require.NoError(t, err)

		accountStatusV3 := environment.NewAccountStatus()
		accountStatusV3.SetAccountPublicKeyCount(2)
		accountStatusV3Bytes := accountStatusV3.ToBytes()
		accountStatusV3Bytes[0] = 0

		registersV3, err := registers.NewByAccountFromPayloads([]*ledger.Payload{
			newPayload(address, flow.AccountStatusKey, accountStatusV3Bytes),
			newPayload(address, legacyAccountPublicKey0RegisterKey, encodedKey0),
			newPayload(address, fmt.Sprintf(legacyAccountPublicKeyRegisterKeyPattern, 1), encodedKey1a),
		})
		require.NoError(t, err)

		accountStatusV4 := environment.NewAccountStatus()
		// Append key 0
		_, _, err = accountStatusV4.AppendAccountPublicKeyMetadata(
			key0.Revoked,
			uint16(key0.Weight),
			encodedStoredKey0,
			func(b []byte) uint64 {
				return accountkeymetadata.GetPublicKeyDigest(address, b)
			},
			func(i uint32) ([]byte, error) {
				return nil, fmt.Errorf("don't expect getStoredKey called, got %d", i)
			},
		)
		require.NoError(t, err)
		// Append key 1
		_, _, err = accountStatusV4.AppendAccountPublicKeyMetadata(
			key1b.Revoked,
			uint16(key1b.Weight),
			encodedStoredKey1b,
			func(b []byte) uint64 {
				return accountkeymetadata.GetPublicKeyDigest(address, b)
			},
			func(i uint32) ([]byte, error) {
				if i == 0 {
					return encodedStoredKey0, nil
				}
				return nil, fmt.Errorf("expect getStoredKey for key index 0, got %d", i)
			},
		)
		require.NoError(t, err)

		registersV4, err := registers.NewByAccountFromPayloads([]*ledger.Payload{
			newPayload(address, flow.AccountStatusKey, accountStatusV4.ToBytes()),
			newPayload(address, "apk_0", encodedKey0),
			newPayload(address, "pk_b0", newBatchPublicKey(t, []*flow.StoredPublicKey{nil, &storedKey1b})),
		})
		require.NoError(t, err)

		reportWriter := newMemoryReportWriter()

		diffReporter := NewAccountKeyDiffReporter(common.Address(address), chainID, reportWriter)
		diffReporter.DiffKeys(registersV3, registersV4)

		require.Equal(t, 1, len(reportWriter.data))
		require.Contains(t, reportWriter.data[0].(accountKeyDiffProblem).Msg, "account public key diff")
	})

	t.Run("2 keys, diff for weight", func(t *testing.T) {
		key0 := newAccountPublicKey(t, 1000)
		storedKey0 := accountPublicKeyToStoredKey(key0)

		encodedKey0, err := flow.EncodeAccountPublicKey(key0)
		require.NoError(t, err)

		encodedStoredKey0, err := flow.EncodeStoredPublicKey(storedKey0)
		require.NoError(t, err)

		key1 := newAccountPublicKey(t, 1)
		storedKey1 := accountPublicKeyToStoredKey(key1)

		encodedKey1, err := flow.EncodeAccountPublicKey(key1)
		require.NoError(t, err)

		encodedStoredKey1, err := flow.EncodeStoredPublicKey(storedKey1)
		require.NoError(t, err)

		accountStatusV3 := environment.NewAccountStatus()
		accountStatusV3.SetAccountPublicKeyCount(2)
		accountStatusV3Bytes := accountStatusV3.ToBytes()
		accountStatusV3Bytes[0] = 0

		registersV3, err := registers.NewByAccountFromPayloads([]*ledger.Payload{
			newPayload(address, flow.AccountStatusKey, accountStatusV3Bytes),
			newPayload(address, legacyAccountPublicKey0RegisterKey, encodedKey0),
			newPayload(address, fmt.Sprintf(legacyAccountPublicKeyRegisterKeyPattern, 1), encodedKey1),
		})
		require.NoError(t, err)

		accountStatusV4 := environment.NewAccountStatus()
		// Append key 0
		_, _, err = accountStatusV4.AppendAccountPublicKeyMetadata(
			key0.Revoked,
			uint16(key0.Weight),
			encodedStoredKey0,
			func(b []byte) uint64 {
				return accountkeymetadata.GetPublicKeyDigest(address, b)
			},
			func(i uint32) ([]byte, error) {
				return nil, fmt.Errorf("don't expect getStoredKey called, got %d", i)
			},
		)
		require.NoError(t, err)
		// Append key 1
		_, _, err = accountStatusV4.AppendAccountPublicKeyMetadata(
			key1.Revoked,
			uint16(key1.Weight+1),
			encodedStoredKey1,
			func(b []byte) uint64 {
				return accountkeymetadata.GetPublicKeyDigest(address, b)
			},
			func(i uint32) ([]byte, error) {
				if i == 0 {
					return encodedStoredKey0, nil
				}
				return nil, fmt.Errorf("expect getStoredKey for key index 0, got %d", i)
			},
		)
		require.NoError(t, err)

		registersV4, err := registers.NewByAccountFromPayloads([]*ledger.Payload{
			newPayload(address, flow.AccountStatusKey, accountStatusV4.ToBytes()),
			newPayload(address, "apk_0", encodedKey0),
			newPayload(address, "pk_b0", newBatchPublicKey(t, []*flow.StoredPublicKey{nil, &storedKey1})),
		})
		require.NoError(t, err)

		reportWriter := newMemoryReportWriter()

		diffReporter := NewAccountKeyDiffReporter(common.Address(address), chainID, reportWriter)
		diffReporter.DiffKeys(registersV3, registersV4)

		require.Equal(t, 1, len(reportWriter.data))
		require.Contains(t, reportWriter.data[0].(accountKeyDiffProblem).Msg, "weight")
	})

	t.Run("2 keys, diff for revoked status", func(t *testing.T) {
		key0 := newAccountPublicKey(t, 1000)
		storedKey0 := accountPublicKeyToStoredKey(key0)

		encodedKey0, err := flow.EncodeAccountPublicKey(key0)
		require.NoError(t, err)

		encodedStoredKey0, err := flow.EncodeStoredPublicKey(storedKey0)
		require.NoError(t, err)

		key1 := newAccountPublicKey(t, 1)
		storedKey1 := accountPublicKeyToStoredKey(key1)

		encodedKey1, err := flow.EncodeAccountPublicKey(key1)
		require.NoError(t, err)

		encodedStoredKey1, err := flow.EncodeStoredPublicKey(storedKey1)
		require.NoError(t, err)

		accountStatusV3 := environment.NewAccountStatus()
		accountStatusV3.SetAccountPublicKeyCount(2)
		accountStatusV3Bytes := accountStatusV3.ToBytes()
		accountStatusV3Bytes[0] = 0

		registersV3, err := registers.NewByAccountFromPayloads([]*ledger.Payload{
			newPayload(address, flow.AccountStatusKey, accountStatusV3Bytes),
			newPayload(address, legacyAccountPublicKey0RegisterKey, encodedKey0),
			newPayload(address, fmt.Sprintf(legacyAccountPublicKeyRegisterKeyPattern, 1), encodedKey1),
		})
		require.NoError(t, err)

		accountStatusV4 := environment.NewAccountStatus()
		// Append key 0
		_, _, err = accountStatusV4.AppendAccountPublicKeyMetadata(
			key0.Revoked,
			uint16(key0.Weight),
			encodedStoredKey0,
			func(b []byte) uint64 {
				return accountkeymetadata.GetPublicKeyDigest(address, b)
			},
			func(i uint32) ([]byte, error) {
				return nil, fmt.Errorf("don't expect getStoredKey called, got %d", i)
			},
		)
		require.NoError(t, err)
		// Append key 1
		_, _, err = accountStatusV4.AppendAccountPublicKeyMetadata(
			!key1.Revoked,
			uint16(key1.Weight),
			encodedStoredKey1,
			func(b []byte) uint64 {
				return accountkeymetadata.GetPublicKeyDigest(address, b)
			},
			func(i uint32) ([]byte, error) {
				if i == 0 {
					return encodedStoredKey0, nil
				}
				return nil, fmt.Errorf("expect getStoredKey for key index 0, got %d", i)
			},
		)
		require.NoError(t, err)

		registersV4, err := registers.NewByAccountFromPayloads([]*ledger.Payload{
			newPayload(address, flow.AccountStatusKey, accountStatusV4.ToBytes()),
			newPayload(address, "apk_0", encodedKey0),
			newPayload(address, "pk_b0", newBatchPublicKey(t, []*flow.StoredPublicKey{nil, &storedKey1})),
		})
		require.NoError(t, err)

		reportWriter := newMemoryReportWriter()

		diffReporter := NewAccountKeyDiffReporter(common.Address(address), chainID, reportWriter)
		diffReporter.DiffKeys(registersV3, registersV4)

		require.Equal(t, 1, len(reportWriter.data))
		require.Contains(t, reportWriter.data[0].(accountKeyDiffProblem).Msg, "revoked status")
	})

	t.Run("2 keys, diff for sequence number", func(t *testing.T) {
		key0 := newAccountPublicKey(t, 1000)
		storedKey0 := accountPublicKeyToStoredKey(key0)

		encodedKey0, err := flow.EncodeAccountPublicKey(key0)
		require.NoError(t, err)

		encodedStoredKey0, err := flow.EncodeStoredPublicKey(storedKey0)
		require.NoError(t, err)

		key1 := newAccountPublicKey(t, 1)
		key1.SeqNumber = 1
		storedKey1 := accountPublicKeyToStoredKey(key1)

		encodedKey1, err := flow.EncodeAccountPublicKey(key1)
		require.NoError(t, err)

		encodedStoredKey1, err := flow.EncodeStoredPublicKey(storedKey1)
		require.NoError(t, err)

		accountStatusV3 := environment.NewAccountStatus()
		accountStatusV3.SetAccountPublicKeyCount(2)
		accountStatusV3Bytes := accountStatusV3.ToBytes()
		accountStatusV3Bytes[0] = 0

		registersV3, err := registers.NewByAccountFromPayloads([]*ledger.Payload{
			newPayload(address, flow.AccountStatusKey, accountStatusV3Bytes),
			newPayload(address, legacyAccountPublicKey0RegisterKey, encodedKey0),
			newPayload(address, fmt.Sprintf(legacyAccountPublicKeyRegisterKeyPattern, 1), encodedKey1),
		})
		require.NoError(t, err)

		accountStatusV4 := environment.NewAccountStatus()
		// Append key 0
		_, _, err = accountStatusV4.AppendAccountPublicKeyMetadata(
			key0.Revoked,
			uint16(key0.Weight),
			encodedStoredKey0,
			func(b []byte) uint64 {
				return accountkeymetadata.GetPublicKeyDigest(address, b)
			},
			func(i uint32) ([]byte, error) {
				return nil, fmt.Errorf("don't expect getStoredKey called, got %d", i)
			},
		)
		require.NoError(t, err)
		// Append key 1
		_, _, err = accountStatusV4.AppendAccountPublicKeyMetadata(
			key1.Revoked,
			uint16(key1.Weight),
			encodedStoredKey1,
			func(b []byte) uint64 {
				return accountkeymetadata.GetPublicKeyDigest(address, b)
			},
			func(i uint32) ([]byte, error) {
				if i == 0 {
					return encodedStoredKey0, nil
				}
				return nil, fmt.Errorf("expect getStoredKey for key index 0, got %d", i)
			},
		)
		require.NoError(t, err)

		registersV4, err := registers.NewByAccountFromPayloads([]*ledger.Payload{
			newPayload(address, flow.AccountStatusKey, accountStatusV4.ToBytes()),
			newPayload(address, "apk_0", encodedKey0),
			newPayload(address, "pk_b0", newBatchPublicKey(t, []*flow.StoredPublicKey{nil, &storedKey1})),
		})
		require.NoError(t, err)

		reportWriter := newMemoryReportWriter()

		diffReporter := NewAccountKeyDiffReporter(common.Address(address), chainID, reportWriter)
		diffReporter.DiffKeys(registersV3, registersV4)

		require.Equal(t, 1, len(reportWriter.data))
		require.Contains(t, reportWriter.data[0].(accountKeyDiffProblem).Msg, "sequence number")
	})

	t.Run("2 keys, diff for hash algo", func(t *testing.T) {
		key0 := newAccountPublicKey(t, 1000)
		storedKey0 := accountPublicKeyToStoredKey(key0)

		encodedKey0, err := flow.EncodeAccountPublicKey(key0)
		require.NoError(t, err)

		encodedStoredKey0, err := flow.EncodeStoredPublicKey(storedKey0)
		require.NoError(t, err)

		key1 := newAccountPublicKey(t, 1)
		storedKey1 := accountPublicKeyToStoredKey(key1)
		storedKey1.HashAlgo = crypto.SHA3_384

		encodedKey1, err := flow.EncodeAccountPublicKey(key1)
		require.NoError(t, err)

		encodedStoredKey1, err := flow.EncodeStoredPublicKey(storedKey1)
		require.NoError(t, err)

		accountStatusV3 := environment.NewAccountStatus()
		accountStatusV3.SetAccountPublicKeyCount(2)
		accountStatusV3Bytes := accountStatusV3.ToBytes()
		accountStatusV3Bytes[0] = 0

		registersV3, err := registers.NewByAccountFromPayloads([]*ledger.Payload{
			newPayload(address, flow.AccountStatusKey, accountStatusV3Bytes),
			newPayload(address, legacyAccountPublicKey0RegisterKey, encodedKey0),
			newPayload(address, fmt.Sprintf(legacyAccountPublicKeyRegisterKeyPattern, 1), encodedKey1),
		})
		require.NoError(t, err)

		accountStatusV4 := environment.NewAccountStatus()
		// Append key 0
		_, _, err = accountStatusV4.AppendAccountPublicKeyMetadata(
			key0.Revoked,
			uint16(key0.Weight),
			encodedStoredKey0,
			func(b []byte) uint64 {
				return accountkeymetadata.GetPublicKeyDigest(address, b)
			},
			func(i uint32) ([]byte, error) {
				return nil, fmt.Errorf("don't expect getStoredKey called, got %d", i)
			},
		)
		require.NoError(t, err)
		// Append key 1
		_, _, err = accountStatusV4.AppendAccountPublicKeyMetadata(
			key1.Revoked,
			uint16(key1.Weight),
			encodedStoredKey1,
			func(b []byte) uint64 {
				return accountkeymetadata.GetPublicKeyDigest(address, b)
			},
			func(i uint32) ([]byte, error) {
				if i == 0 {
					return encodedStoredKey0, nil
				}
				return nil, fmt.Errorf("expect getStoredKey for key index 0, got %d", i)
			},
		)
		require.NoError(t, err)

		registersV4, err := registers.NewByAccountFromPayloads([]*ledger.Payload{
			newPayload(address, flow.AccountStatusKey, accountStatusV4.ToBytes()),
			newPayload(address, "apk_0", encodedKey0),
			newPayload(address, "pk_b0", newBatchPublicKey(t, []*flow.StoredPublicKey{nil, &storedKey1})),
		})
		require.NoError(t, err)

		reportWriter := newMemoryReportWriter()

		diffReporter := NewAccountKeyDiffReporter(common.Address(address), chainID, reportWriter)
		diffReporter.DiffKeys(registersV3, registersV4)

		require.Equal(t, 1, len(reportWriter.data))
		require.Contains(t, reportWriter.data[0].(accountKeyDiffProblem).Msg, "hash algo")
	})

	t.Run("2 duplicate keys", func(t *testing.T) {
		key0 := newAccountPublicKey(t, 1000)
		storedKey0 := accountPublicKeyToStoredKey(key0)

		encodedKey0, err := flow.EncodeAccountPublicKey(key0)
		require.NoError(t, err)

		encodedStoredKey0, err := flow.EncodeStoredPublicKey(storedKey0)
		require.NoError(t, err)

		key1 := key0
		key1.SeqNumber = 1
		storedKey1 := accountPublicKeyToStoredKey(key1)

		encodedKey1, err := flow.EncodeAccountPublicKey(key1)
		require.NoError(t, err)

		encodedStoredKey1, err := flow.EncodeStoredPublicKey(storedKey1)
		require.NoError(t, err)

		accountStatusV3 := environment.NewAccountStatus()
		accountStatusV3.SetAccountPublicKeyCount(2)
		accountStatusV3Bytes := accountStatusV3.ToBytes()
		accountStatusV3Bytes[0] = 0

		registersV3, err := registers.NewByAccountFromPayloads([]*ledger.Payload{
			newPayload(address, flow.AccountStatusKey, accountStatusV3Bytes),
			newPayload(address, fmt.Sprintf(legacyAccountPublicKeyRegisterKeyPattern, 0), encodedKey0),
			newPayload(address, fmt.Sprintf(legacyAccountPublicKeyRegisterKeyPattern, 1), encodedKey1),
		})
		require.NoError(t, err)

		accountStatusV4 := environment.NewAccountStatus()
		// Append key 0
		_, _, err = accountStatusV4.AppendAccountPublicKeyMetadata(
			key0.Revoked,
			uint16(key0.Weight),
			encodedStoredKey0,
			func(b []byte) uint64 {
				return accountkeymetadata.GetPublicKeyDigest(address, b)
			},
			func(i uint32) ([]byte, error) {
				return nil, fmt.Errorf("don't expect getStoredKey called, got %d", i)
			},
		)
		require.NoError(t, err)
		// Append key 1
		_, _, err = accountStatusV4.AppendAccountPublicKeyMetadata(
			key1.Revoked,
			uint16(key1.Weight),
			encodedStoredKey1,
			func(b []byte) uint64 {
				return accountkeymetadata.GetPublicKeyDigest(address, b)
			},
			func(i uint32) ([]byte, error) {
				if i == 0 {
					return encodedStoredKey0, nil
				}
				return nil, fmt.Errorf("expect getStoredKey for key index 0, got %d", i)
			},
		)
		require.NoError(t, err)

		encodedSeqNumber, err := flow.EncodeSequenceNumber(key1.SeqNumber)
		require.NoError(t, err)

		registersV4, err := registers.NewByAccountFromPayloads([]*ledger.Payload{
			newPayload(address, flow.AccountStatusKey, accountStatusV4.ToBytes()),
			newPayload(address, "apk_0", encodedKey0),
			newPayload(address, "sn_1", encodedSeqNumber),
		})
		require.NoError(t, err)

		reportWriter := newMemoryReportWriter()

		diffReporter := NewAccountKeyDiffReporter(common.Address(address), chainID, reportWriter)
		diffReporter.DiffKeys(registersV3, registersV4)

		// No diff
		require.Equal(t, 0, len(reportWriter.data))
	})
}

func newPayload(owner flow.Address, key string, value []byte) *ledger.Payload {
	registerID := flow.NewRegisterID(owner, key)
	ledgerKey := convert.RegisterIDToLedgerKey(registerID)
	return ledger.NewPayload(ledgerKey, value)
}

type memoryReportWriter struct {
	data []any
}

func newMemoryReportWriter() *memoryReportWriter {
	return &memoryReportWriter{}
}

func (w *memoryReportWriter) Write(dataPoint interface{}) {
	w.data = append(w.data, dataPoint)
}

func (w *memoryReportWriter) Close() {
}

func accountPublicKeyToStoredKey(apk flow.AccountPublicKey) flow.StoredPublicKey {
	return flow.StoredPublicKey{
		PublicKey: apk.PublicKey,
		SignAlgo:  apk.SignAlgo,
		HashAlgo:  apk.HashAlgo,
	}
}

func newBatchPublicKey(t *testing.T, storedPublicKeys []*flow.StoredPublicKey) []byte {
	var buf []byte
	var err error

	for _, k := range storedPublicKeys {
		var encodedKey []byte

		if k != nil {
			encodedKey, err = flow.EncodeStoredPublicKey(*k)
			require.NoError(t, err)
		}

		b, err := encodeBatchedPublicKey(encodedKey)
		require.NoError(t, err)

		buf = append(buf, b...)
	}
	return buf
}

func encodeBatchedPublicKey(encodedPublicKey []byte) ([]byte, error) {
	const maxEncodedKeySize = math.MaxUint8

	if len(encodedPublicKey) > maxEncodedKeySize {
		return nil, fmt.Errorf("failed to encode batched public key: encoded key size is %d bytes, exceeded max size %d", len(encodedPublicKey), maxEncodedKeySize)
	}

	buf := make([]byte, 1+len(encodedPublicKey))
	buf[0] = byte(len(encodedPublicKey))
	copy(buf[1:], encodedPublicKey)

	return buf, nil
}
