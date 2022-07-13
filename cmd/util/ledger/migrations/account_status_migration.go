package migrations

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/rs/zerolog"

	"github.com/onflow/atree"
	"github.com/onflow/flow-go/engine/execution/state"
	fvmState "github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
)

const (
	KeyExists          = "exists"
	KeyAccountFrozen   = "frozen"
	KeyPublicKeyCount  = "public_key_count"
	KeyStorageUsed     = "storage_used"
	KeyStorageIndex    = "storage_index"
	KeyPrefixPublicKey = "public_key_"
)

// AccountStatusMigration migrates previous registers under
// key of Exists which were used for checking existance of accounts.
// the new register AccountStatus also captures frozen and all future states
// of the accounts. Frozen state is used when an account is set
// by the network governance for furture investigation and prevents operations on the account until
// furthure investigation by the community.
// This migration assumes no account has been frozen until now, and would warn if
// find any account with frozen flags.
type AccountStatusMigration struct {
	logger    zerolog.Logger
	statuses  map[string]fvmState.AccountStatus
	keyCounts map[string]uint64
}

func NewAccountStatusMigration(logger zerolog.Logger) *AccountStatusMigration {
	return &AccountStatusMigration{
		logger:    logger,
		statuses:  make(map[string]fvmState.AccountStatus),
		keyCounts: make(map[string]uint64),
	}
}

func (as *AccountStatusMigration) getStatus(owner []byte) fvmState.AccountStatus {
	st, exist := as.statuses[string(owner)]
	if !exist {
		return fvmState.NewAccountStatus()
	}
	return st
}

func (as *AccountStatusMigration) setStatus(owner []byte, st fvmState.AccountStatus) {
	as.statuses[string(owner)] = st
}

func (as *AccountStatusMigration) getKeyCount(owner []byte) uint64 {
	return as.keyCounts[string(owner)]
}

func (as *AccountStatusMigration) setKeyCount(owner []byte, count uint64) {
	as.keyCounts[string(owner)] = count
}

func (as *AccountStatusMigration) Migrate(payload []ledger.Payload) ([]ledger.Payload, error) {
	newPayloads := make([]ledger.Payload, 0, len(payload))

	for _, p := range payload {
		owner := p.Key.KeyParts[0].Value
		key := p.Key.KeyParts[2].Value

		switch string(key) {
		case KeyExists:
			// in case there are cases that doesn't have other registers
			st := as.getStatus(owner)
			as.setStatus(owner, st)
		case KeyPublicKeyCount:
			// following the original way of decoding of value
			countInt := new(big.Int).SetBytes(p.Value)
			count := countInt.Uint64()
			status := as.getStatus(owner)
			status.SetPublicKeyCount(count)
			as.setStatus(owner, status)
		case KeyStorageUsed:
			// following the original way of decoding of value
			if len(p.Value) < 8 {
				return nil, fmt.Errorf("malsized storage used, owner: %s value: %s", hex.EncodeToString(owner), hex.EncodeToString(p.Value))
			}
			used := binary.BigEndian.Uint64(p.Value[:8])
			status := as.getStatus(owner)
			status.SetStorageUsed(used)
			as.setStatus(owner, status)
		case KeyStorageIndex:
			if len(p.Value) < 8 {
				return nil, fmt.Errorf("malsized storage index, owner: %s value: %s", hex.EncodeToString(owner), hex.EncodeToString(p.Value))
			}
			var index atree.StorageIndex
			copy(index[:], p.Value[:8])
			status := as.getStatus(owner)
			status.SetStorageIndex(index)
			as.setStatus(owner, status)
		case KeyAccountFrozen:
			status := as.getStatus(owner)
			status.SetFrozenFlag(true)
			as.setStatus(owner, status)

		default: // else just append and continue
			if strings.HasPrefix(string(key), KeyPrefixPublicKey) {
				as.setKeyCount(owner, as.getKeyCount(owner)+1)
			}
			newPayloads = append(newPayloads, p)
		}
	}

	for owner, status := range as.statuses {
		// sanity check on key counts (if they don't align might cause real issues)
		if status.PublicKeyCount() != as.getKeyCount([]byte(owner)) {
			return nil, fmt.Errorf("public key count doesn't match, owner: %s count from status: %d, count from registers: %d",
				hex.EncodeToString([]byte(owner)),
				status.PublicKeyCount(),
				as.getKeyCount([]byte(owner)))
		}
		newKey := ledger.NewKey([]ledger.KeyPart{
			ledger.NewKeyPart(state.KeyPartOwner, []byte(owner)),
			ledger.NewKeyPart(1, []byte("")), // legacy controller
			ledger.NewKeyPart(state.KeyPartKey, []byte(fvmState.KeyAccountStatus)),
		})
		newPayloads = append(newPayloads, *ledger.NewPayload(newKey, status.ToBytes()))
	}
	return newPayloads, nil
}
