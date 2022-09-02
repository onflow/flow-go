package migrations

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/onflow/atree"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/fvm/environment"
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
	statuses  map[string]*environment.AccountStatus
	keyCounts map[string]uint64
}

func NewAccountStatusMigration(logger zerolog.Logger) *AccountStatusMigration {
	return &AccountStatusMigration{
		logger:    logger,
		statuses:  make(map[string]*environment.AccountStatus),
		keyCounts: make(map[string]uint64),
	}
}

func (as *AccountStatusMigration) getOrInitStatus(owner []byte) *environment.AccountStatus {
	st, exist := as.statuses[string(owner)]
	if !exist {
		return environment.NewAccountStatus()
	}
	return st
}

func (as *AccountStatusMigration) setStatus(owner []byte, st *environment.AccountStatus) {
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
		k, err := p.Key()
		if err != nil {
			return nil, err
		}
		owner := k.KeyParts[0].Value
		key := k.KeyParts[2].Value

		switch string(key) {
		case KeyExists:
			// in case an account doesn't have other registers
			// we need to consturct an status anyway
			st := as.getOrInitStatus(owner)
			as.setStatus(owner, st)
		case KeyPublicKeyCount:
			// follow the original way of decoding the value
			countInt := new(big.Int).SetBytes(p.Value())
			count := countInt.Uint64()
			// update status
			status := as.getOrInitStatus(owner)
			status.SetPublicKeyCount(count)
			as.setStatus(owner, status)
		case KeyStorageUsed:
			// follow the original way of decoding the value
			if len(p.Value()) < 8 {
				return nil, fmt.Errorf("malsized storage used, owner: %s value: %s", hex.EncodeToString(owner), hex.EncodeToString(p.Value()))
			}
			used := binary.BigEndian.Uint64(p.Value()[:8])
			// update status
			status := as.getOrInitStatus(owner)
			status.SetStorageUsed(used)
			as.setStatus(owner, status)
		case KeyStorageIndex:
			// follow the original way of decoding the value
			if len(p.Value()) < 8 {
				return nil, fmt.Errorf("malsized storage index, owner: %s value: %s", hex.EncodeToString(owner), hex.EncodeToString(p.Value()))
			}
			var index atree.StorageIndex
			copy(index[:], p.Value()[:8])
			// update status
			status := as.getOrInitStatus(owner)
			status.SetStorageIndex(index)
			as.setStatus(owner, status)
		case KeyAccountFrozen:
			status := as.getOrInitStatus(owner)
			status.SetFrozenFlag(true)
			as.setStatus(owner, status)
		default: // else just append and continue
			// collect actual public keys per accounts for a final sanity check
			if strings.HasPrefix(string(key), KeyPrefixPublicKey) {
				as.setKeyCount(owner, as.getKeyCount(owner)+1)
			}
			newPayloads = append(newPayloads, p)
		}
	}

	// instead of removed registers add status to accounts
	for owner, status := range as.statuses {
		// sanity check on key counts (if it doesn't match, it could cause real issues (e.g. key override))
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
