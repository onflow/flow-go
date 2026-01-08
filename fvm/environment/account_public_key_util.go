package environment

import (
	"fmt"
	"math"
	"slices"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
)

const (
	// NOTE: MaxPublicKeyCountInBatch can't be modified
	MaxPublicKeyCountInBatch = 20 // 20 public key payload is ~1420 bytes
)

// Account Public Key 0

func getAccountPublicKey0(
	a Accounts,
	address flow.Address,
) (
	flow.AccountPublicKey,
	error,
) {
	encodedPublicKey, err := a.GetValue(flow.AccountPublicKey0RegisterID(address))
	if err != nil {
		return flow.AccountPublicKey{}, err
	}

	const keyIndex = uint32(0)

	if len(encodedPublicKey) == 0 {
		return flow.AccountPublicKey{}, errors.NewAccountPublicKeyNotFoundError(
			address,
			keyIndex)
	}

	decodedPublicKey, err := flow.DecodeAccountPublicKey(encodedPublicKey, keyIndex)
	if err != nil {
		return flow.AccountPublicKey{}, fmt.Errorf(
			"failed to decode account public key 0: %w",
			err)
	}

	return decodedPublicKey, nil
}

func setAccountPublicKey0(
	a Accounts,
	address flow.Address,
	publicKey flow.AccountPublicKey,
) error {
	err := publicKey.Validate()
	if err != nil {
		encoded, _ := publicKey.MarshalJSON()
		return errors.NewValueErrorf(
			string(encoded),
			"invalid public key value: %w",
			err)
	}

	encodedPublicKey, err := flow.EncodeAccountPublicKey(publicKey)
	if err != nil {
		encoded, _ := publicKey.MarshalJSON()
		return errors.NewValueErrorf(
			string(encoded),
			"failed to encode account public key: %w",
			err)
	}

	err = a.SetValue(
		flow.AccountPublicKey0RegisterID(address),
		encodedPublicKey)

	return err
}

func revokeAccountPublicKey0(
	a Accounts,
	address flow.Address,
) error {
	accountPublicKey0RegisterID := flow.AccountPublicKey0RegisterID(address)

	publicKey, err := a.GetValue(accountPublicKey0RegisterID)
	if err != nil {
		return err
	}

	const keyIndex = uint32(0)

	if len(publicKey) == 0 {
		return errors.NewAccountPublicKeyNotFoundError(
			address,
			keyIndex)
	}

	decodedPublicKey, err := flow.DecodeAccountPublicKey(publicKey, keyIndex)
	if err != nil {
		return fmt.Errorf(
			"failed to decode account public key 0: %w",
			err)
	}

	decodedPublicKey.Revoked = true

	encodedPublicKey, err := flow.EncodeAccountPublicKey(decodedPublicKey)
	if err != nil {
		encoded, _ := decodedPublicKey.MarshalJSON()
		return errors.NewValueErrorf(
			string(encoded),
			"failed to encode revoked account public key 0: %w",
			err)
	}

	return a.SetValue(accountPublicKey0RegisterID, encodedPublicKey)
}

// Account Public Key Sequence Number

func getAccountPublicKeySequenceNumber(
	a Accounts,
	address flow.Address,
	keyIndex uint32,
) (uint64, error) {
	if keyIndex == 0 {
		decodedAccountPublicKey, err := getAccountPublicKey0(a, address)
		if err != nil {
			return 0, err
		}
		return decodedAccountPublicKey.SeqNumber, nil
	}

	encodedSequenceNumber, err := a.GetValue(flow.AccountPublicKeySequenceNumberRegisterID(address, keyIndex))
	if err != nil {
		return 0, err
	}

	if len(encodedSequenceNumber) == 0 {
		return 0, nil
	}

	return flow.DecodeSequenceNumber(encodedSequenceNumber)
}

func incrementAccountPublicKeySequenceNumber(
	a Accounts,
	address flow.Address,
	keyIndex uint32,
) error {
	if keyIndex == 0 {
		decodedAccountPublicKey, err := getAccountPublicKey0(a, address)
		if err != nil {
			return err
		}

		decodedAccountPublicKey.SeqNumber++

		return setAccountPublicKey0(a, address, decodedAccountPublicKey)
	}

	seqNum, err := getAccountPublicKeySequenceNumber(a, address, keyIndex)
	if err != nil {
		return err
	}

	seqNum++

	return createAccountPublicKeySequenceNumber(a, address, keyIndex, seqNum)
}

func createAccountPublicKeySequenceNumber(
	a Accounts,
	address flow.Address,
	keyIndex uint32,
	seqNum uint64,
) error {
	if keyIndex == 0 {
		return errors.NewKeyMetadataUnexpectedKeyIndexError("failed to create sequence number register", keyIndex)
	}

	encodedSeqNum, err := flow.EncodeSequenceNumber(seqNum)
	if err != nil {
		return err
	}

	return a.SetValue(
		flow.AccountPublicKeySequenceNumberRegisterID(address, keyIndex),
		encodedSeqNum)
}

// Batch Public Key

// BatchPublicKey register contains up to maxPublicKeyCountInBatch number of encoded public keys.
// Each public key is encoded as:
// - length prefixed encoded stored public key

func getStoredPublicKey(
	a Accounts,
	address flow.Address,
	storedKeyIndex uint32,
) (flow.StoredPublicKey, error) {
	if storedKeyIndex == 0 {
		// Stored key 0 is always account public key 0.

		accountKey, err := getAccountPublicKey0(a, address)
		if err != nil {
			return flow.StoredPublicKey{}, err
		}

		return flow.StoredPublicKey{
			PublicKey: accountKey.PublicKey,
			SignAlgo:  accountKey.SignAlgo,
			HashAlgo:  accountKey.HashAlgo,
		}, nil
	}

	encodedPublicKey, err := getRawStoredPublicKey(a, address, storedKeyIndex)
	if err != nil {
		return flow.StoredPublicKey{}, err
	}

	return flow.DecodeStoredPublicKey(encodedPublicKey)
}

func getRawStoredPublicKey(
	a Accounts,
	address flow.Address,
	storedKeyIndex uint32,
) ([]byte, error) {

	if storedKeyIndex == 0 {
		// Stored key index is always account public key 0

		accountKey, err := getAccountPublicKey0(a, address)
		if err != nil {
			return nil, err
		}

		storedKey := flow.StoredPublicKey{
			PublicKey: accountKey.PublicKey,
			SignAlgo:  accountKey.SignAlgo,
			HashAlgo:  accountKey.HashAlgo,
		}

		return flow.EncodeStoredPublicKey(storedKey)
	}

	batchIndex := storedKeyIndex / MaxPublicKeyCountInBatch
	keyIndexInBatch := storedKeyIndex % MaxPublicKeyCountInBatch

	batchRegisterKey := flow.AccountBatchPublicKeyRegisterID(address, batchIndex)

	b, err := a.GetValue(batchRegisterKey)
	if err != nil {
		return nil, err
	}

	if len(b) == 0 {
		return nil, errors.NewBatchPublicKeyNotFoundError("failed to get stored public key", address, batchIndex)
	}

	for off, i := 0, uint32(0); off < len(b); i++ {
		size := int(b[off])
		off++

		if off+size > len(b) {
			return nil, errors.NewBatchPublicKeyDecodingError(
				fmt.Sprintf("%s register is too short", batchRegisterKey),
				address,
				batchIndex)
		}

		if i == keyIndexInBatch {
			encodedPublicKey := b[off : off+size]
			return slices.Clone(encodedPublicKey), nil
		}

		off += size
	}

	return nil, errors.NewStoredPublicKeyNotFoundError(
		fmt.Sprintf("%s register doesn't have key at index %d", batchRegisterKey, keyIndexInBatch),
		address,
		storedKeyIndex)
}

func appendStoredKey(
	a Accounts,
	address flow.Address,
	storedKeyIndex uint32,
	encodedPublicKey []byte,
) error {
	if storedKeyIndex == 0 {
		return errors.NewStoredPublicKeyUnexpectedIndexError("failed to append stored key 0 to batch public key", address, storedKeyIndex)
	}

	encodedBatchedPublicKey, err := encodeBatchedPublicKey(encodedPublicKey)
	if err != nil {
		return err
	}

	batchNum := storedKeyIndex / MaxPublicKeyCountInBatch
	indexInBatch := storedKeyIndex % MaxPublicKeyCountInBatch

	batchPublicKeyRegisterKey := flow.AccountBatchPublicKeyRegisterID(address, batchNum)

	if indexInBatch == 0 {
		// Create new batch public key with 1 key
		return a.SetValue(batchPublicKeyRegisterKey, encodedBatchedPublicKey)
	}

	if batchNum == 0 && indexInBatch == 1 {
		// Create new batch public key with 2 keys:
		// - key 0 is nil (placeholder for account public key 0)
		// - key 1 is the new key
		// NOTE: key 0 in batch 0 is encoded as nil because key 0 is already stored in apk_0 register.

		var batchPublicKeyBytes []byte
		batchPublicKeyBytes = append(batchPublicKeyBytes, encodedNilBatchedPublicKey...)
		batchPublicKeyBytes = append(batchPublicKeyBytes, encodedBatchedPublicKey...)

		return a.SetValue(batchPublicKeyRegisterKey, batchPublicKeyBytes)
	}

	existingBatchKeyPayload, err := a.GetValue(batchPublicKeyRegisterKey)
	if err != nil {
		return err
	}
	if len(existingBatchKeyPayload) == 0 {
		return errors.NewBatchPublicKeyNotFoundError("failed to append stored public key", address, batchNum)
	}

	// Append new key to existing batch public key register
	batchPublicKeyBytes := append([]byte(nil), existingBatchKeyPayload...)
	batchPublicKeyBytes = append(batchPublicKeyBytes, encodedBatchedPublicKey...)

	return a.SetValue(batchPublicKeyRegisterKey, batchPublicKeyBytes)
}

var encodedNilBatchedPublicKey, _ = encodeBatchedPublicKey(nil)

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

// Utility functions for tests and validation

// DecodeBatchPublicKeys decodes raw bytes of batch public keys in the batch public key payload.
func DecodeBatchPublicKeys(b []byte) ([][]byte, error) {
	if len(b) == 0 {
		return nil, nil
	}

	encodedPublicKeys := make([][]byte, 0, MaxPublicKeyCountInBatch)

	off := 0
	for off < len(b) {
		size := int(b[off])
		off++

		if off+size > len(b) {
			return nil, errors.NewCodedFailuref(
				errors.FailureCodeBatchPublicKeyDecodingFailure,
				"failed to decode batch public key",
				"batch public key data is too short: %x",
				b,
			)
		}

		encodedPublicKey := b[off : off+size]
		off += size

		encodedPublicKeys = append(encodedPublicKeys, encodedPublicKey)
	}

	if off != len(b) {
		return nil, errors.NewCodedFailuref(
			errors.FailureCodeBatchPublicKeyDecodingFailure,
			"failed to decode batch public key",
			"batch public key data has trailiing data (%d bytes): %x",
			len(b)-off,
			b,
		)
	}

	return encodedPublicKeys, nil
}
