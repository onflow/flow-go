package badger

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
)

const (
	blockKeyPrefix           = "block_by_number"
	blockHashIndexKeyPrefix  = "block_hash_to_number"
	transactionKeyPrefix     = "transaction_by_hash"
	ledgerKeyPrefix          = "ledger_by_block_number" // TODO remove
	eventsKeyPrefix          = "events_by_block_number"
	ledgerChangelogKeyPrefix = "ledger_changelog_by_register_id"
	ledgerValueKeyPrefix     = "ledger_value_by_block_number_register_id"
)

// The following *Key functions return keys to use when reading/writing values
// to Badger. The key name describes how the value is indexed (eg. by block
// number or by hash).
//
// Keys for which numeric ordering is defined, (eg. block number), have the
// numeric component of the key left-padded with zeros (%032d) so that
// lexicographic ordering matches numeric ordering.

func blockKey(blockNumber uint64) []byte {
	return []byte(fmt.Sprintf("%s-%032d", blockKeyPrefix, blockNumber))
}

func blockHashIndexKey(blockHash crypto.Hash) []byte {
	return []byte(fmt.Sprintf("%s-%s", blockHashIndexKeyPrefix, blockHash.Hex()))
}

func latestBlockKey() []byte {
	return []byte("latest_block_number")
}

func transactionKey(txHash crypto.Hash) []byte {
	return []byte(fmt.Sprintf("%s-%s", transactionKeyPrefix, txHash.Hex()))
}

func eventsKey(blockNumber uint64) []byte {
	return []byte(fmt.Sprintf("%s-%032d", eventsKeyPrefix, blockNumber))
}

// TODO remove this
func ledgerKey(blockNumber uint64) []byte {
	return []byte(fmt.Sprintf("%s-%032d", ledgerKeyPrefix, blockNumber))
}

func ledgerChangelogKey(registerID string) []byte {
	return []byte(fmt.Sprintf("%s-%s", ledgerChangelogKeyPrefix, registerID))
}

func ledgerValueKey(registerID string, blockNumber uint64) []byte {
	return []byte(fmt.Sprintf("%s-%s-%032d", ledgerValueKeyPrefix, registerID, blockNumber))
}

// blockNumberFromEventsKey recovers the block number from an event key.
func blockNumberFromEventsKey(key []byte) uint64 {
	var blockNumber uint64
	_, _ = fmt.Sscanf(string(key), eventsKeyPrefix+"-%032d", &blockNumber)
	return blockNumber
}

// registerIDFromLedgerChangelogKey recovers the register ID from a ledger
// changelog key.
func registerIDFromLedgerChangelogKey(key []byte) string {
	var registerID string
	_, _ = fmt.Sscanf(string(key), ledgerChangelogKeyPrefix+"-%s", &registerID)
	return registerID
}
