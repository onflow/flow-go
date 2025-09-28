package migrations

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"path"
	"sync"
	"time"

	"github.com/fxamacker/circlehash"
	"github.com/rs/zerolog"

	"github.com/onflow/cadence/common"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/model/flow"
)

// NOTE: The term "payload" and "register" are used interchangeably here.

// Public key deduplication migration deduplicates public keys and migrates related payloads.
// Migration includes:
// - Optionally appending account public key metadata in "a.s" (account status) payload.
//   - Weight and revoked status of each account public key, encoded using RLE to save space.
//     The weight cannot be modified and revoke status is infrequently modified.
//   - Key index mapping to stored key index mapping (only for accounts with duplication flag)
//     encoded in RLE to save space.
//   - Last N digests to detect duplicate keys being added at runtime (after migration and spork).
// - Renaming the payload key from "public_key_0" to "apk_0" without changing the payload value.
// - Migrating public keys from individual payloads to batch deduplicated public key payloads,
//   starting from the second unique public key.
// - Migrating non-zero sequence number of account public key to its own payload.
//   NOTE: We store and update the sequence number of each account public key in a separate payload
//   to avoid blocking some use cases of concurrent execution.
//
// Using a data format (account public key metadata) that can detect duplicates and store deduplication data
// requires storing some related information (overhead) but in most cases the overhead is more than offset
// by deduplication.
// To avoid or reduce overhead,
// - migration only adds key metadata section to "a.s" payload for accounts with at least two keys.
// - migration only stores digests of the last N unique public keys (N=2 is good, using more wasn't always better).
// - migration only stores account public keys to stored public keys mappings if key deduplication occurred.
//
// More specifically:
// - For accounts with 0 public keys, migration skips them
// - For accounts with 1 public key, migration only renames the "public_key_0" payload to "apk_0" (no other changes)
// - For accounts with at least two keys, migration:
//   * renames the "public_key_0" payload to "apk_0"
//   * stores unique keys in batch public key payload, starting from public key 1
//   * stores non-zero sequence numbers in sequence number payloads
//   * adds account key weights and revoked statuses to the key metadata section in key metadata section
//   * adds digests of only the last N unique public keys in key metadata section (N=2 is the default)
//   * adds account public key to unique key mappings if any key is deduplicated

const (
	legacyAccountPublicKeyRegisterKeyPrefix  = "public_key_"
	legacyAccountPublicKeyRegisterKeyPattern = "public_key_%d"
	legacyAccountPublicKey0RegisterKey       = "public_key_0"
)

const (
	maxPublicKeyCountInBatch = 20 // 20 public key payload is ~1420 bytes
	maxStoredDigests         = 2  // Account status payload stores up to 2 digests from last 2 stored keys.
)

const (
	dummyDigest = uint64(0)
)

// AccountPublicKeyDeduplicationMigration deduplicates account public keys,
// and migrates account status and account public key related payloads.
type AccountPublicKeyDeduplicationMigration struct {
	log                     zerolog.Logger
	chainID                 flow.ChainID
	outputDir               string
	reporter                reporters.ReportWriter
	validationReporter      reporters.ReportWriter
	migrationResult         migrationResult
	accountMigrationResults []accountMigrationResult
	resultLock              sync.Mutex
	validate                bool
}

var _ AccountBasedMigration = (*AccountPublicKeyDeduplicationMigration)(nil)

func NewAccountPublicKeyDeduplicationMigration(
	chainID flow.ChainID,
	outputDir string,
	validate bool,
	rwf reporters.ReportWriterFactory,
) *AccountPublicKeyDeduplicationMigration {

	m := &AccountPublicKeyDeduplicationMigration{
		chainID:   chainID,
		reporter:  rwf.ReportWriter("account-public-key-deduplication-migration_summary"),
		outputDir: outputDir,
		validate:  validate,
	}

	if validate {
		m.validationReporter = rwf.ReportWriter("account-public-key-deduplication-validation")
	}

	return m
}

func (m *AccountPublicKeyDeduplicationMigration) InitMigration(
	log zerolog.Logger,
	registersByAccount *registers.ByAccount,
	_ int,
) error {
	m.log = log.With().Str("component", "DeduplicateAccountPublicKey").Logger()
	m.accountMigrationResults = make([]accountMigrationResult, 0, registersByAccount.AccountCount())
	return nil
}

func (m *AccountPublicKeyDeduplicationMigration) MigrateAccount(
	_ context.Context,
	address common.Address,
	accountRegisters *registers.AccountRegisters,
) error {
	beforeCount := accountRegisters.Count()
	beforeSize := accountRegisters.PayloadSize()

	deduplicated, err := migrateAndDeduplicateAccountPublicKeys(m.log, accountRegisters)
	if err != nil {
		return fmt.Errorf("failed to migrate and deduplicate account public keys for account %x: %w", accountRegisters.Owner(), err)
	}

	if m.validate {
		err := ValidateAccountPublicKeyV4(address, accountRegisters)
		if err != nil {
			m.validationReporter.Write(validationError{
				Address: address.Hex(),
				Msg:     err.Error(),
			})
		}
	}

	afterCount := accountRegisters.Count()
	afterSize := accountRegisters.PayloadSize()

	migrationResult := accountMigrationResult{
		address:      hex.EncodeToString([]byte(accountRegisters.Owner())),
		beforeCount:  beforeCount,
		beforeSize:   beforeSize,
		afterCount:   afterCount,
		afterSize:    afterSize,
		deduplicated: deduplicated,
	}

	m.resultLock.Lock()
	defer m.resultLock.Unlock()

	m.accountMigrationResults = append(m.accountMigrationResults, migrationResult)

	if deduplicated {
		m.migrationResult.TotalDeduplicatedAccountCount++
	} else {
		m.migrationResult.TotalUndeduplicatedAccountCount++
	}

	m.migrationResult.TotalSizeDelta += afterSize - beforeSize
	m.migrationResult.TotalCountDelta += afterCount - beforeCount

	return nil
}

func (m *AccountPublicKeyDeduplicationMigration) Close() error {
	// Write migration summary
	m.reporter.Write(m.migrationResult)
	defer m.reporter.Close()

	// Write account migration results
	fileName := path.Join(m.outputDir, fmt.Sprintf("%s_%d.csv", "account_public_key_deduplication_account_migration_results", int32(time.Now().Unix())))
	return writeAccountMigrationResults(fileName, m.accountMigrationResults)
}

func migrateAndDeduplicateAccountPublicKeys(
	log zerolog.Logger,
	accountRegisters *registers.AccountRegisters,
) (deduplicated bool, _ error) {

	owner := accountRegisters.Owner()

	encodedAccountStatusV4, err := migrateAccountStatusToV4(log, accountRegisters, owner)
	if err != nil {
		return false, fmt.Errorf("failed to migrate account status from v3 to v4 for %x: %w", owner, err)
	}

	accountStatusV4, err := environment.AccountStatusFromBytes(encodedAccountStatusV4)
	if err != nil {
		return false, fmt.Errorf("failed to create AccountStatus from migrated payload for %x: %w", owner, err)
	}

	accountPublicKeyCount := accountStatusV4.AccountPublicKeyCount()

	if accountPublicKeyCount == 0 {
		return false, nil
	}

	if accountPublicKeyCount == 1 {
		_, err := migrateAccountPublicKey0(accountRegisters, owner)
		if err != nil {
			return false, fmt.Errorf("failed to migrate account public key 0 for %x: %w", owner, err)
		}
		return false, nil
	}

	encodedAccountPublicKey0, err := migrateAccountPublicKey0(accountRegisters, owner)
	if err != nil {
		return false, fmt.Errorf("failed to migrate account public key 0 for %x: %w", owner, err)
	}

	return migrateAndDeduplicateAccountPublicKeysIfNeeded(
		log,
		accountRegisters,
		owner,
		accountPublicKeyCount,
		encodedAccountPublicKey0,
	)
}

func migrateAndDeduplicateAccountPublicKeysIfNeeded(
	log zerolog.Logger,
	accountRegisters *registers.AccountRegisters,
	owner string,
	accountPublicKeyCount uint32,
	encodedAccountPublicKey0 []byte,
) (
	deduplicated bool,
	err error,
) {
	// TODO: maybe special case migration for accounts with 2 account public keys (16% of accounts)

	// accountPublicKeyWeightAndRevokedStatuses is ordered by account public key index,
	// starting from account public key at index 1.
	accountPublicKeyWeightAndRevokedStatuses := make([]accountPublicKeyWeightAndRevokedStatus, 0, accountPublicKeyCount-1)

	// Account public key deduplicator deduplicates keys.
	deduplicator := newAccountPublicKeyDeduplicator(owner, accountPublicKeyCount)

	// Add account public key 0 to deduplicator
	err = deduplicator.addEncodedAccountPublicKey(0, encodedAccountPublicKey0)
	if err != nil {
		return false, fmt.Errorf("failed to add account public key at index %d for owner %x to deduplicator: %w", 0, owner, err)
	}

	for keyIndex := uint32(1); keyIndex < accountPublicKeyCount; keyIndex++ {

		decodedAccountPublicKey, err := getAccountPublicKeyOrError(accountRegisters, owner, keyIndex)
		if err != nil {
			return false, fmt.Errorf("failed to decode public key at index %d for owner %x: %w", keyIndex, owner, err)
		}

		err = deduplicator.addAccountPublicKey(keyIndex, decodedAccountPublicKey)
		if err != nil {
			return false, fmt.Errorf("failed to add account public key at index %d for owner %x to deduplicator: %w", keyIndex, owner, err)
		}

		// Save weight and revoked status for account public key
		accountPublicKeyWeightAndRevokedStatuses = append(
			accountPublicKeyWeightAndRevokedStatuses,
			accountPublicKeyWeightAndRevokedStatus{
				weight:  uint16(decodedAccountPublicKey.Weight),
				revoked: decodedAccountPublicKey.Revoked,
			},
		)

		// Migrate sequence number for account public key
		// NOTE: sequence number is stored in its own payload, decoupled from public key.
		err = migrateSeqNumberIfNeeded(accountRegisters, owner, keyIndex, decodedAccountPublicKey.SeqNumber)
		if err != nil {
			return false, fmt.Errorf("failed to migrate sequence number at index %d for owner %x: %w", keyIndex, owner, err)
		}

		// Remove account public key from storage
		// NOTE: account public key starting from key index 1 stores in batch.
		err = removeAccountPublicKey(accountRegisters, owner, keyIndex)
		if err != nil {
			return false, fmt.Errorf("failed to remove public key at index %d for owner %x: %w", keyIndex, owner, err)
		}
	}

	shouldDeduplicate := deduplicator.hasDuplicateKey()

	encodedPublicKeys := deduplicator.uniqueKeys()

	// Migrate account status with account public key metadata
	err = migrateAccountStatusWithPublicKeyMetadata(
		log,
		accountRegisters,
		owner,
		accountPublicKeyWeightAndRevokedStatuses,
		encodedPublicKeys,
		deduplicator.keyIndexMapping(),
		shouldDeduplicate,
	)
	if err != nil {
		return false, fmt.Errorf("failed to migrate account status with key metadata for account %x: %w", owner, err)
	}

	// Migrate account public key at index >= 1
	err = migrateAccountPublicKeysIfNeeded(accountRegisters, owner, encodedPublicKeys)
	if err != nil {
		return false, fmt.Errorf("failed to migrate account public keys in batches for account %x: %w", owner, err)
	}

	return shouldDeduplicate, nil
}

// accountPublicKeyDeduplicator deduplicates all account public keys (including account public key 0).
type accountPublicKeyDeduplicator struct {
	owner string

	accountPublicKeyCount uint32

	// uniqEncodedPublicKeysMap is used to deduplicate encoded public key.
	uniqEncodedPublicKeysMap map[string]uint32 // key: encoded public key, value: index of uniqEncodedPublicKeys

	// uniqEncodedPublicKeys contains unique encoded public key.
	// NOTE: First element is always encoded account public key 0.
	uniqEncodedPublicKeys [][]byte

	// accountPublicKeyIndexMappings contains mapping of account public key index to unique public key index.
	// NOTE: First mapping is always 0 (account public key index 0) to 0 (unique key index 0).
	accountPublicKeyIndexMappings []uint32 // index: account public key index, element: uniqEncodedPublicKeys index
}

func newAccountPublicKeyDeduplicator(owner string, accountPublicKeyCount uint32) *accountPublicKeyDeduplicator {
	return &accountPublicKeyDeduplicator{
		owner:                         owner,
		accountPublicKeyCount:         accountPublicKeyCount,
		uniqEncodedPublicKeysMap:      make(map[string]uint32),
		uniqEncodedPublicKeys:         make([][]byte, 0, accountPublicKeyCount),
		accountPublicKeyIndexMappings: make([]uint32, accountPublicKeyCount),
	}
}

func (d *accountPublicKeyDeduplicator) addEncodedAccountPublicKey(
	keyIndex uint32,
	encodedAccountPublicKey []byte,
) error {
	accountPublicKey0, err := flow.DecodeAccountPublicKey(encodedAccountPublicKey, keyIndex)
	if err != nil {
		return fmt.Errorf("failed to decode account public key %d for owner %x: %w", keyIndex, d.owner, err)
	}
	return d.addAccountPublicKey(keyIndex, accountPublicKey0)
}

func (d *accountPublicKeyDeduplicator) addAccountPublicKey(
	keyIndex uint32,
	accountPublicKey flow.AccountPublicKey,
) error {
	encodedPublicKey, err := encodeStoredPublicKeyFromAccountPublicKey(accountPublicKey)
	if err != nil {
		return fmt.Errorf("failed to encode stored public key at index %d for owner %x: %w", keyIndex, d.owner, err)
	}

	if uniqKeyIndex, exists := d.uniqEncodedPublicKeysMap[string(encodedPublicKey)]; !exists {
		// New key is unique

		// Append key to unique key list
		d.uniqEncodedPublicKeys = append(d.uniqEncodedPublicKeys, encodedPublicKey)

		// Unique key index is the last key index in uniqEncodedPublicKeys.
		uniqKeyIndex = uint32(len(d.uniqEncodedPublicKeys) - 1)

		// Append unique key index to account public key mappings
		d.accountPublicKeyIndexMappings[keyIndex] = uniqKeyIndex

		d.uniqEncodedPublicKeysMap[string(encodedPublicKey)] = uniqKeyIndex
	} else {
		// New key is duplicate
		d.accountPublicKeyIndexMappings[keyIndex] = uniqKeyIndex
	}

	return nil
}

func (d *accountPublicKeyDeduplicator) hasDuplicateKey() bool {
	return d.accountPublicKeyCount > uint32(len(d.uniqEncodedPublicKeys))
}

func (d *accountPublicKeyDeduplicator) keyIndexMapping() []uint32 {
	return d.accountPublicKeyIndexMappings
}

func (d *accountPublicKeyDeduplicator) uniqueKeys() [][]byte {
	return d.uniqEncodedPublicKeys
}

func generateLastNPublicKeyDigests(
	log zerolog.Logger,
	owner string,
	encodedPublicKeys [][]byte,
	n int,
	hashFunc func(b []byte, seed uint64) uint64,
) (int, []uint64) {
	digestCount := min(len(encodedPublicKeys), n)
	startIndex := len(encodedPublicKeys) - digestCount
	digests := generatePublicKeyDigests(log, owner, encodedPublicKeys[startIndex:], hashFunc)
	return startIndex, digests
}

// generatePublicKeyDigests returns digests of encodedPublicKeys.
func generatePublicKeyDigests(
	log zerolog.Logger,
	owner string,
	encodedPublicKeys [][]byte,
	hashFunc func(b []byte, seed uint64) uint64,
) (digests []uint64) {
	if hashFunc == nil {
		hashFunc = circlehash.Hash64
	}

	seed := binary.BigEndian.Uint64([]byte(owner))

	digests = make([]uint64, len(encodedPublicKeys))

	collisions := make(map[uint64][]int)
	hasCollision := false

	for i, encodedPublicKey := range encodedPublicKeys {
		digest := hashFunc(encodedPublicKey, seed)

		if _, exists := collisions[digest]; exists {
			hasCollision = true
			digests[i] = dummyDigest
		} else {
			digests[i] = digest
		}

		collisions[digest] = append(collisions[digest], i)
	}

	if hasCollision {
		for digest, encodedPublicKeyIndexes := range collisions {
			if len(encodedPublicKeyIndexes) > 1 {
				log.Warn().Msgf("found digest collisions for account %x: digest %d, encoded public key indexes %v", owner, digest, encodedPublicKeyIndexes)
			}
		}
	}

	return digests
}

type validationError struct {
	Address string
	Msg     string
}
