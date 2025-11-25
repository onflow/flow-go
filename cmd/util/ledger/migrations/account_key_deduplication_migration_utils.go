package migrations

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/model/flow"
)

const (
	accountStatusV4WithNoDeduplicationFlag = 0x40
)

func getAccountRegisterOrError(
	accountRegisters *registers.AccountRegisters,
	owner string,
	key string,
) ([]byte, error) {
	value, err := accountRegisters.Get(owner, key)
	if err != nil {
		return nil, err
	}
	if len(value) == 0 {
		return nil, fmt.Errorf("owner %x key %s register not found", owner, key)
	}
	return value, nil
}

func getAccountPublicKeyOrError(
	accountRegisters *registers.AccountRegisters,
	owner string,
	keyIndex uint32,
) (flow.AccountPublicKey, error) {
	publicKeyRegisterKey := fmt.Sprintf(legacyAccountPublicKeyRegisterKeyPattern, keyIndex)

	encodedAccountPublicKey, err := getAccountRegisterOrError(accountRegisters, owner, publicKeyRegisterKey)
	if err != nil {
		return flow.AccountPublicKey{}, err
	}

	decodedAccountPublicKey, err := flow.DecodeAccountPublicKey(encodedAccountPublicKey, keyIndex)
	if err != nil {
		return flow.AccountPublicKey{}, err
	}

	return decodedAccountPublicKey, nil
}

func removeAccountPublicKey(
	accountRegisters *registers.AccountRegisters,
	owner string,
	keyIndex uint32,
) error {
	publicKeyRegisterKey := fmt.Sprintf(legacyAccountPublicKeyRegisterKeyPattern, keyIndex)
	return accountRegisters.Set(owner, publicKeyRegisterKey, nil)
}

// migrateAccountStatusToV4 sets account status version to v4, stores account status
// in account status register, and returns updated account status.
func migrateAccountStatusToV4(
	log zerolog.Logger,
	accountRegisters *registers.AccountRegisters,
	owner string,
) ([]byte, error) {
	encodedAccountStatus, err := getAccountRegisterOrError(accountRegisters, owner, flow.AccountStatusKey)
	if err != nil {
		return nil, err
	}

	if encodedAccountStatus[0] != 0 {
		log.Warn().Msgf("%x account status flag is %d, flag will be reset during migration", owner, encodedAccountStatus[0])
	}

	// Update account status version and flag in place.
	encodedAccountStatus[0] = accountStatusV4WithNoDeduplicationFlag

	// Set account status register
	err = accountRegisters.Set(owner, flow.AccountStatusKey, encodedAccountStatus)
	if err != nil {
		return nil, err
	}

	return encodedAccountStatus, nil
}

// migrateAccountPublicKey0 renames public_key_0 to apk_0, stores renamed
// account public key, and returns raw data of account public key 0.
func migrateAccountPublicKey0(
	accountRegisters *registers.AccountRegisters,
	owner string,
) ([]byte, error) {
	encodedAccountPublicKey0, err := getAccountRegisterOrError(accountRegisters, owner, legacyAccountPublicKey0RegisterKey)
	if err != nil {
		return nil, err
	}

	// Rename public_key_0 register key to apk_0
	err = accountRegisters.Set(owner, legacyAccountPublicKey0RegisterKey, nil)
	if err != nil {
		return nil, err
	}
	err = accountRegisters.Set(owner, flow.AccountPublicKey0RegisterKey, encodedAccountPublicKey0)
	if err != nil {
		return nil, err
	}

	return encodedAccountPublicKey0, nil
}

// migrateSeqNumberIfNeeded creates sequence number register for the given
// account public key if sequence number is > 0.
func migrateSeqNumberIfNeeded(
	accountRegisters *registers.AccountRegisters,
	owner string,
	keyIndex uint32,
	seqNumber uint64,
) error {
	if seqNumber == 0 {
		return nil
	}

	seqNumberRegisterKey := fmt.Sprintf(flow.SequenceNumberRegisterKeyPattern, keyIndex)

	encodedSeqNumber, err := flow.EncodeSequenceNumber(seqNumber)
	if err != nil {
		return err
	}

	return accountRegisters.Set(owner, seqNumberRegisterKey, encodedSeqNumber)
}

// migrateAccountStatusWithPublicKeyMetadata appends accounts public key metadata
// to account status register.
func migrateAccountStatusWithPublicKeyMetadata(
	log zerolog.Logger,
	accountRegisters *registers.AccountRegisters,
	owner string,
	accountPublicKeyWeightAndRevokedStatus []accountPublicKeyWeightAndRevokedStatus,
	encodedPublicKeys [][]byte,
	keyIndexMappings []uint32,
	deduplicated bool,
) error {
	encodedAccountStatus, err := getAccountRegisterOrError(accountRegisters, owner, flow.AccountStatusKey)
	if err != nil {
		return err
	}

	// After the migration and spork, the runtime needs to detect duplicate public keys
	// being added and store them efficiently. Detection rate doesn't need to be 100%
	// to be effective and we don't want to read all existing keys or store all digests.
	// We only need to compute and store the hash digest of the last N public keys added.
	// For example, N=2 showed good balance of tradeoffs in tests using mainnet snapshot.
	startIndexForDigests, digests := generateLastNPublicKeyDigests(log, owner, encodedPublicKeys, maxStoredDigests, nil)

	// startIndexForMapping stores the start index of the first deduplicated public key.
	// This is used to avoid unnecessary key index mapping overhead (both speed & storage).
	startIndexForMapping := firstIndexOfDuplicateKeyInMappings(keyIndexMappings)

	// keyIndexMappings is a slice containing stored key index where the slice index is
	// the account key index starting from startIndexForMapping.
	keyIndexMappings = keyIndexMappings[startIndexForMapping:]

	newAccountStatus, err := encodeAccountStatusV4WithPublicKeyMetadata(
		encodedAccountStatus,
		accountPublicKeyWeightAndRevokedStatus,
		uint32(startIndexForDigests),
		digests,
		uint32(startIndexForMapping),
		keyIndexMappings,
		deduplicated,
	)
	if err != nil {
		return err
	}

	return accountRegisters.Set(owner, flow.AccountStatusKey, newAccountStatus)
}

// migrateAccountPublicKeysIfNeeded migrates account public key from index >= 1 to batched public key register.
// NOTE:
// - Key 0 in batch 0 is always empty since it corresponds to account public key 0, which is stored in its own register.
func migrateAccountPublicKeysIfNeeded(
	accountRegisters *registers.AccountRegisters,
	owner string,
	encodedUniquePublicKeys [][]byte,
) error {
	// Return early if encodedPublicKeys only contains public key 0 (which always stores in register apk_0)
	if len(encodedUniquePublicKeys) == 1 {
		return nil
	}

	// Storing public keys in batch reduces payload count and reduces number of reads for multiple public keys.
	// About 65% of accounts have 1 public key so the first public key is not deduplicated to avoid overhead.
	// About 90% of accounts have fewer than 10 account public keys, so with batching 90% of accounts have at
	// most 2 payloads for public keys (since the first key is always by itself to avoid overhead).
	// - apk_0 payload for account public key 0, and
	// - pb_b0 payload for rest of deduplicated public keys
	encodedBatchPublicKeys, err := encodePublicKeysInBatches(encodedUniquePublicKeys, maxPublicKeyCountInBatch)
	if err != nil {
		return err
	}

	for batchIndex, encodedBatchPublicKey := range encodedBatchPublicKeys {
		batchPublicKeyRegisterKey := fmt.Sprintf(flow.BatchPublicKeyRegisterKeyPattern, batchIndex)
		err = accountRegisters.Set(owner, batchPublicKeyRegisterKey, encodedBatchPublicKey)
		if err != nil {
			return err
		}
	}

	return nil
}

func firstIndexOfDuplicateKeyInMappings(accountPublicKeyMappings []uint32) int {
	for keyIndex, storedKeyIndex := range accountPublicKeyMappings {
		if uint32(keyIndex) != storedKeyIndex {
			return keyIndex
		}
	}
	return len(accountPublicKeyMappings)
}
