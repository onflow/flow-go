package migrations

import (
	"encoding/binary"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/onflow/cadence/common"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/model/flow"
)

func validateAccountPublicKeyV4(
	address common.Address,
	accountRegisters *registers.AccountRegisters,
) error {

	// Validate account status register
	accountPublicKeyCount, storedKeyCount, deduplicated, err := validateAccountStatusV4Register(address, accountRegisters)
	if err != nil {
		return err
	}

	if deduplicated && accountPublicKeyCount == storedKeyCount {
		return fmt.Errorf("stored key count is the same as account key count when account is deduplicated: %d keys", accountPublicKeyCount)
	}

	// Find relevant registers
	foundAccountPublicKey0 := false
	sequeceNumberRegisters := make(map[string][]byte)
	batchPublicKeyRegisters := make(map[string][]byte)

	err = accountRegisters.ForEach(func(_ string, key string, value []byte) error {
		if strings.HasPrefix(key, legacyAccountPublicKeyRegisterKeyPrefix) {
			return fmt.Errorf("found legacy account public key register %s", key)
		} else if key == flow.AccountPublicKey0RegisterKey {
			foundAccountPublicKey0 = true
		} else if strings.HasPrefix(key, flow.BatchPublicKeyRegisterKeyPrefix) {
			batchPublicKeyRegisters[key] = value
		} else if strings.HasPrefix(key, flow.SequenceNumberRegisterKeyPrefix) {
			sequeceNumberRegisters[key] = value
		}
		return nil
	})
	if err != nil {
		return err
	}

	if foundAccountPublicKey0 && accountPublicKeyCount == 0 {
		return fmt.Errorf("found account public key 0 when account public key count is 0")
	}

	if !foundAccountPublicKey0 && accountPublicKeyCount > 0 {
		return fmt.Errorf("no account public key 0 when account public key count is %d", accountPublicKeyCount)
	}

	if accountPublicKeyCount <= 1 {
		if len(sequeceNumberRegisters) != 0 {
			return fmt.Errorf("found %d sequence number register when account public key count is %d", len(sequeceNumberRegisters), accountPublicKeyCount)
		}
		if len(batchPublicKeyRegisters) != 0 {
			return fmt.Errorf("found %d batch public key register when account public key count is %d", len(batchPublicKeyRegisters), accountPublicKeyCount)
		}
		return nil
	}

	// Check sequence number registers.
	err = validateSequenceNumberRegisters(accountPublicKeyCount, sequeceNumberRegisters)
	if err != nil {
		return err
	}

	// Check batch public key registers.
	return validateBatchPublicKeyRegisters(storedKeyCount, batchPublicKeyRegisters)
}

func validateAccountStatusV4Register(
	address common.Address,
	accountRegisters *registers.AccountRegisters,
) (
	accountPublicKeyCount uint32,
	storedKeyCount uint32,
	deduplicated bool,
	err error,
) {
	owner := string(address[:])

	encodedAccountStatus, err := accountRegisters.Get(owner, flow.AccountStatusKey)
	if err != nil {
		err = fmt.Errorf("failed to get account status register: %w", err)
		return
	}

	if len(encodedAccountStatus) == 0 {
		err = fmt.Errorf("account status register is empty")
		return
	}

	accountStatus, err := environment.AccountStatusFromBytes(encodedAccountStatus)
	if err != nil {
		err = fmt.Errorf("failed to create account status bytes %x: %w", encodedAccountStatus, err)
		return
	}

	if accountStatus.Version() != 4 {
		err = fmt.Errorf("account status version is %d", accountStatus.Version())
		return
	}

	deduplicated = accountStatus.IsAccountKeyDeduplicated()

	accountPublicKeyCount = accountStatus.AccountPublicKeyCount()

	if accountPublicKeyCount <= 1 {
		if len(encodedAccountStatus) != environment.AccountStatusMinSizeV4 {
			err = fmt.Errorf("account status register size is %d, expect %d bytes", len(encodedAccountStatus), environment.AccountStatusMinSizeV4)
			return
		}
		storedKeyCount = accountPublicKeyCount
		return
	}

	weightAndRevokedStatus, startKeyIndexForMapping, accountPublicKeyMappings, startKeyIndexForDigests, digests, err := decodeAccountStatusKeyMetadata(
		encodedAccountStatus[environment.AccountStatusMinSizeV4:],
		deduplicated,
	)
	if err != nil {
		err = fmt.Errorf("failed to parse key metadata %x: %s", encodedAccountStatus[environment.AccountStatusMinSizeV4:], err)
		return
	}

	if !deduplicated && len(accountPublicKeyMappings) > 0 {
		err = fmt.Errorf("expect no mapping when account is not deduplicated, got %v", accountPublicKeyMappings)
		return
	}

	storedKeyCount = uint32(len(digests)) + startKeyIndexForDigests

	err = validateKeyMetadata(
		deduplicated,
		accountPublicKeyCount,
		weightAndRevokedStatus,
		startKeyIndexForDigests,
		digests,
		startKeyIndexForMapping,
		accountPublicKeyMappings,
	)
	if err != nil {
		return
	}

	return
}

func validateSequenceNumberRegisters(
	accountPublicKeyCount uint32,
	sequeceNumberRegisters map[string][]byte,
) error {
	if len(sequeceNumberRegisters) > int(accountPublicKeyCount-1) {
		return fmt.Errorf("found %d sequence number register when account public key count is %d", len(sequeceNumberRegisters), accountPublicKeyCount)
	}

	for i := 1; i < int(accountPublicKeyCount); i++ {
		key := fmt.Sprintf(flow.SequenceNumberRegisterKeyPattern, i)
		encoded, exists := sequeceNumberRegisters[key]
		if exists {
			sequenceNumber, err := flow.DecodeSequenceNumber(encoded)
			if err != nil {
				return fmt.Errorf("failed to decode sequence number register %s, %x: %s", key, encoded, err)
			}
			if sequenceNumber == 0 {
				return fmt.Errorf("found sequence number 0 in sequence number register %s", key)
			}
			delete(sequeceNumberRegisters, key)
		}
	}

	if len(sequeceNumberRegisters) != 0 {
		return fmt.Errorf("found %d unexpected sequence number registers: %+v", len(sequeceNumberRegisters), slices.Collect(maps.Keys(sequeceNumberRegisters)))
	}

	return nil
}

func validateBatchPublicKeyRegisters(
	storedKeyCount uint32,
	batchPublicKeyRegisters map[string][]byte,
) error {
	if storedKeyCount == 1 {
		if len(batchPublicKeyRegisters) > 0 {
			return fmt.Errorf("found %d batch public key payloads while stored key count is %d", len(batchPublicKeyRegisters), storedKeyCount)
		}
		return nil
	}

	keyCount := 0

	for batchNum := range len(batchPublicKeyRegisters) {
		key := fmt.Sprintf(flow.BatchPublicKeyRegisterKeyPattern, batchNum)

		encoded, exists := batchPublicKeyRegisters[key]
		if !exists {
			return fmt.Errorf("failed to find batch public key %s", key)
		}

		encodedKeys, err := decodeBatchPublicKey(encoded)
		if err != nil {
			return fmt.Errorf("failed to decode batch public key register %s, %x: %s", key, encoded, err)
		}

		batchCount := len(encodedKeys)
		keyCount += batchCount

		if batchCount == 0 {
			return fmt.Errorf("found batch public key %s with 0 keys", key)
		}

		if batchNum == 0 {
			if len(encodedKeys[0]) != 0 {
				return fmt.Errorf("found unexpected key 0 at batch 0: %x", encodedKeys[0])
			}

			if len(encodedKeys) == 1 {
				return fmt.Errorf("found only key 0 in batch public key 0")
			}

			encodedKeys = encodedKeys[1:]
		}

		for _, encodedKey := range encodedKeys {
			_, err = flow.DecodeStoredPublicKey(encodedKey)
			if err != nil {
				return fmt.Errorf("failed to decode stored public key %x in register %s: %s", encodedKey, key, err)
			}
		}

		if batchCount < maxPublicKeyCountInBatch && batchNum != len(batchPublicKeyRegisters)-1 {
			return fmt.Errorf("batch public key %s has less than max count in a batch: got %d keys, %d batches in total", key, batchCount, len(batchPublicKeyRegisters))
		}
	}

	if keyCount != int(storedKeyCount) {
		return fmt.Errorf("found %d stored keys in batch public key registers vs stored key count %d in key metadata ", keyCount, storedKeyCount)
	}

	return nil
}

func validateKeyMetadata(
	deduplicated bool,
	accountPublicKeyCount uint32,
	weightAndRevokedStatus []accountPublicKeyWeightAndRevokedStatus,
	startKeyIndexForDigests uint32,
	digests []uint64,
	startKeyIndexForMapping uint32,
	accountPublicKeyMappings []uint32,
) error {
	if len(weightAndRevokedStatus) != int(accountPublicKeyCount)-1 {
		return fmt.Errorf("found %d weight and revoked status, expect %d", len(weightAndRevokedStatus), accountPublicKeyCount-1)
	}

	if len(digests) > maxStoredDigests {
		return fmt.Errorf("found %d digests, expect max %d digests", len(digests), maxStoredDigests)
	}

	if len(digests) > int(accountPublicKeyCount) {
		return fmt.Errorf("found %d digest, expect fewer digests than account public key count %d", len(digests), accountPublicKeyCount)
	}

	if int(startKeyIndexForDigests)+len(digests) > int(accountPublicKeyCount) {
		return fmt.Errorf("found %d digest at start index %d, expect fewer digests than account public key count %d", len(digests), startKeyIndexForDigests, accountPublicKeyCount)
	}

	if deduplicated {
		if int(startKeyIndexForMapping)+len(accountPublicKeyMappings) != int(accountPublicKeyCount) {
			return fmt.Errorf("found %d mappings at start index %d, expect %d",
				len(accountPublicKeyMappings),
				startKeyIndexForMapping,
				accountPublicKeyCount,
			)
		}
	} else {
		if len(accountPublicKeyMappings) > 0 {
			return fmt.Errorf("found %d account public key mappings for non-deduplicated account, expect 0", len(accountPublicKeyMappings))
		}
	}

	return nil
}

// Decode util functions used for testing and migration validation

func decodeAccountPublicKeyWeightAndRevokedStatusGroups(b []byte) ([]accountPublicKeyWeightAndRevokedStatus, error) {
	if len(b)%weightAndRevokedStatusGroupSize != 0 {
		return nil, fmt.Errorf("failed to decode weight and revoked status: expect multiple of %d bytes, got %d", weightAndRevokedStatusGroupSize, len(b))
	}

	statuses := make([]accountPublicKeyWeightAndRevokedStatus, 0, len(b)/weightAndRevokedStatusGroupSize)

	for i := 0; i < len(b); i += weightAndRevokedStatusGroupSize {
		runLength := uint32(binary.BigEndian.Uint16(b[i:]))
		weightAndRevoked := binary.BigEndian.Uint16(b[i+2 : i+4])

		status := accountPublicKeyWeightAndRevokedStatus{
			weight:  weightAndRevoked & weightMask,
			revoked: (weightAndRevoked & revokedMask) > 0,
		}

		for range runLength {
			statuses = append(statuses, status)
		}
	}

	return statuses, nil
}

func decodeAccountPublicKeyMapping(b []byte) ([]uint32, error) {
	if len(b)%mappingGroupSize != 0 {
		return nil, fmt.Errorf("failed to decode mappings: expect multiple of %d bytes, got %d", mappingGroupSize, len(b))
	}

	mapping := make([]uint32, 0, len(b)/mappingGroupSize)

	for i := 0; i < len(b); i += mappingGroupSize {
		runLength := binary.BigEndian.Uint16(b[i:])
		storedKeyIndex := binary.BigEndian.Uint32(b[i+runLengthSize:])

		if highBit := (runLength & consecutiveGroupFlagMask) >> 15; highBit == 1 {
			runLength &= lengthMask

			for i := range runLength {
				mapping = append(mapping, storedKeyIndex+uint32(i))
			}
		} else {
			for range runLength {
				mapping = append(mapping, storedKeyIndex)
			}
		}
	}

	return mapping, nil
}

func decodeDigestList(b []byte) ([]uint64, error) {
	if len(b)%digestSize != 0 {
		return nil, fmt.Errorf("failed to decode digest list: expect multiple of %d byte, got %d", digestSize, len(b))
	}

	storedDigestCount := len(b) / digestSize

	digests := make([]uint64, 0, storedDigestCount)

	for i := 0; i < len(b); i += digestSize {
		digests = append(digests, binary.BigEndian.Uint64(b[i:]))
	}

	return digests, nil
}

func decodeBatchPublicKey(b []byte) ([][]byte, error) {
	if len(b) == 0 {
		return nil, nil
	}

	encodedPublicKeys := make([][]byte, 0, maxPublicKeyCountInBatch)

	off := 0
	for off < len(b) {
		size := int(b[off])
		off++

		if off+size > len(b) {
			return nil, fmt.Errorf("failed to decode batch public key: off %d + size %d out of bounds %d: %x", off, size, len(b), b)
		}

		encodedPublicKey := b[off : off+size]
		off += size

		encodedPublicKeys = append(encodedPublicKeys, encodedPublicKey)
	}

	if off != len(b) {
		return nil, fmt.Errorf("failed to decode batch public key: trailing data (%d bytes): %x", len(b)-off, b)
	}

	return encodedPublicKeys, nil
}

func decodeAccountStatusKeyMetadata(b []byte, deduplicated bool) (
	weightAndRevokedStatus []accountPublicKeyWeightAndRevokedStatus,
	startKeyIndexForMapping uint32,
	accountPublicKeyMappings []uint32,
	startKeyIndexForDigests uint32,
	digests []uint64,
	err error,
) {
	// Decode weight and revoked list

	var weightAndRevokedGroupsData []byte
	weightAndRevokedGroupsData, b, err = parseNextLengthPrefixedData(b)
	if err != nil {
		err = fmt.Errorf("failed to decode AccountStatusV4: %w", err)
		return
	}

	weightAndRevokedStatus, err = decodeAccountPublicKeyWeightAndRevokedStatusGroups(weightAndRevokedGroupsData)
	if err != nil {
		err = fmt.Errorf("failed to decode weight and revoked status list: %w", err)
		return
	}

	// Decode account public key mapping if deduplication is on

	if deduplicated {
		if len(b) < 4 {
			err = fmt.Errorf("failed to decode AccountStatusV4: expect 4 bytes of start key index for mapping, got %d bytes", len(b))
			return
		}

		startKeyIndexForMapping = binary.BigEndian.Uint32(b)

		b = b[4:]

		var mappingData []byte
		mappingData, b, err = parseNextLengthPrefixedData(b)
		if err != nil {
			err = fmt.Errorf("failed to decode AccountStatusV4: %w", err)
			return
		}

		accountPublicKeyMappings, err = decodeAccountPublicKeyMapping(mappingData)
		if err != nil {
			err = fmt.Errorf("failed to decode account public key mappings: %w", err)
			return
		}
	}

	// Decode digests list

	if len(b) < 4 {
		err = fmt.Errorf("failed to decode AccountStatusV4: expect 4 bytes of start stored key index for digests, got %d bytes", len(b))
		return
	}

	startKeyIndexForDigests = binary.BigEndian.Uint32(b)
	b = b[4:]

	var digestsData []byte
	digestsData, b, err = parseNextLengthPrefixedData(b)
	if err != nil {
		err = fmt.Errorf("failed to decode AccountStatusV4: %w", err)
		return
	}

	digests, err = decodeDigestList(digestsData)
	if err != nil {
		err = fmt.Errorf("failed to decode digests: %w", err)
		return
	}

	// Check trailing data

	if len(b) != 0 {
		err = fmt.Errorf("failed to decode AccountStatusV4: got %d extra bytes", len(b))
		return
	}

	return
}

func parseNextLengthPrefixedData(b []byte) (next []byte, rest []byte, err error) {
	if len(b) < lengthPrefixSize {
		return nil, nil, fmt.Errorf("failed to decode data: expect at least 4 bytes, got %d bytes", len(b))
	}

	length := binary.BigEndian.Uint32(b[:lengthPrefixSize])

	if len(b) < lengthPrefixSize+int(length) {
		return nil, nil, fmt.Errorf("failed to decode data: expect at least %d bytes, got %d bytes", lengthPrefixSize+int(length), len(b))
	}

	b = b[lengthPrefixSize:]
	return b[:length], b[length:], nil
}
