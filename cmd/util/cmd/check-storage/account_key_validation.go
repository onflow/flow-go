package check_storage

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/onflow/cadence/common"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	accountkeymetadata "github.com/onflow/flow-go/fvm/environment/account-key-metadata"
	"github.com/onflow/flow-go/model/flow"
)

func validateAccountPublicKey(
	address common.Address,
	accountRegisters *registers.AccountRegisters,
) error {
	// Skip empty address because it doesn't have account status register.
	if len(address) == 0 || address == common.ZeroAddress {
		return nil
	}

	// Validate account status register
	accountPublicKeyCount, storedKeyCount, deduplicated, err := validateAccountStatusRegister(address, accountRegisters)
	if err != nil {
		return err
	}

	if storedKeyCount > accountPublicKeyCount {
		return fmt.Errorf("number of stored keys shouldn't be greater than number of account keys, got %d stored keys, and %d account keys", storedKeyCount, accountPublicKeyCount)
	}

	if deduplicated && accountPublicKeyCount == storedKeyCount {
		return fmt.Errorf("number of deduplicated stored keys shouldn't equal number of account keys, got %d stored keys, and %d account keys", storedKeyCount, accountPublicKeyCount)
	}

	// Find relevant registers
	foundAccountPublicKey0 := false
	sequeceNumberRegisters := make(map[string][]byte)
	batchPublicKeyRegisters := make(map[string][]byte)

	err = accountRegisters.ForEach(func(_ string, key string, value []byte) error {
		if key == flow.AccountPublicKey0RegisterKey {
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

func validateAccountStatusRegister(
	address common.Address,
	accountRegisters *registers.AccountRegisters,
) (
	accountPublicKeyCount uint32,
	storedKeyCount uint32,
	deduplicated bool,
	err error,
) {
	owner := flow.AddressToRegisterOwner(flow.Address(address[:]))

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
		if len(encodedAccountStatus) != environment.AccountStatusMinSize {
			err = fmt.Errorf("account status register size is %d, expect %d bytes", len(encodedAccountStatus), environment.AccountStatusMinSize)
			return
		}
		storedKeyCount = accountPublicKeyCount
		return
	}

	weightAndRevokedStatus, startKeyIndexForMapping, accountPublicKeyMappings, startKeyIndexForDigests, digests, err := accountkeymetadata.DecodeKeyMetadata(
		encodedAccountStatus[environment.AccountStatusMinSize:],
		deduplicated,
	)
	if err != nil {
		err = fmt.Errorf("failed to parse key metadata %x: %s", encodedAccountStatus[environment.AccountStatusMinSize:], err)
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

		encodedKeys, err := environment.DecodeBatchPublicKeys(encoded)
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

		if batchCount < environment.MaxPublicKeyCountInBatch && batchNum != len(batchPublicKeyRegisters)-1 {
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
	weightAndRevokedStatus []accountkeymetadata.WeightAndRevokedStatus,
	startKeyIndexForDigests uint32,
	digests []uint64,
	startKeyIndexForMapping uint32,
	accountPublicKeyMappings []uint32,
) error {
	if len(weightAndRevokedStatus) != int(accountPublicKeyCount)-1 {
		return fmt.Errorf("found %d weight and revoked status, expect %d", len(weightAndRevokedStatus), accountPublicKeyCount-1)
	}

	if len(digests) > environment.MaxStoredDigests {
		return fmt.Errorf("found %d digests, expect max %d digests", len(digests), environment.MaxStoredDigests)
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
