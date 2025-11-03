package migrations

import (
	"encoding/json"
	"fmt"

	"github.com/onflow/cadence/common"

	"github.com/onflow/crypto/hash"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/model/flow"
)

type accountKeyDiffKind int

const (
	accountKeyCountDiff accountKeyDiffKind = iota
	accountKeyDiff
)

var accountKeyDiffKindString = map[accountKeyDiffKind]string{
	accountKeyCountDiff: "key_count_diff",
	accountKeyDiff:      "key_diff",
}

type accountKeyDiffProblem struct {
	Address  string
	KeyIndex uint32
	Kind     string
	Msg      string
}

type accountKeyDiffErrorKind int

const (
	accountKeyCountErrorKind accountKeyDiffErrorKind = iota
	accountKeyErrorKind
)

var accountKeyDiffErrorKindString = map[accountKeyDiffErrorKind]string{
	accountKeyCountErrorKind: "error_get_key_count_failed",
	accountKeyErrorKind:      "error_get_key_failed",
}

type accountKeyDiffError struct {
	Address string
	Kind    string
	Msg     string
}

type AccountKeyDiffReporter struct {
	address      common.Address
	chainID      flow.ChainID
	reportWriter reporters.ReportWriter
}

func NewAccountKeyDiffReporter(
	address common.Address,
	chainID flow.ChainID,
	rw reporters.ReportWriter,
) *AccountKeyDiffReporter {
	return &AccountKeyDiffReporter{
		address:      address,
		chainID:      chainID,
		reportWriter: rw,
	}
}

func (akd *AccountKeyDiffReporter) DiffKeys(
	accountRegistersV3, accountRegistersV4 registers.Registers,
) {
	if akd.address == common.ZeroAddress {
		return
	}

	accountsV3 := newAccountsWithAccountKeyV3(accountRegistersV3)

	accountsV4 := newAccountsWithAccountKeyV4(accountRegistersV4)

	countV3, err := accountsV3.getAccountPublicKeyCount(flow.Address(akd.address))
	if err != nil {
		akd.reportWriter.Write(
			accountKeyDiffError{
				Address: akd.address.Hex(),
				Kind:    accountKeyDiffErrorKindString[accountKeyCountErrorKind],
				Msg:     fmt.Sprintf("failed to get account public key count v3: %s", err),
			})
		return
	}

	countV4, err := accountsV4.getAccountPublicKeyCount(flow.Address(akd.address))
	if err != nil {
		akd.reportWriter.Write(
			accountKeyDiffError{
				Address: akd.address.Hex(),
				Kind:    accountKeyDiffErrorKindString[accountKeyCountErrorKind],
				Msg:     fmt.Sprintf("failed to get account public key count v4: %s", err),
			})
		return
	}

	if countV3 != countV4 {
		akd.reportWriter.Write(
			accountKeyDiffProblem{
				Address: akd.address.Hex(),
				Kind:    accountKeyDiffKindString[accountKeyCountDiff],
				Msg:     fmt.Sprintf("%d keys in v3, %d keys in v4", countV3, countV4),
			})
		return
	}

	for keyIndex := range countV3 {
		keyV3, err := accountsV3.getAccountPublicKey(flow.Address(akd.address), keyIndex)
		if err != nil {
			akd.reportWriter.Write(
				accountKeyDiffError{
					Address: akd.address.Hex(),
					Kind:    accountKeyDiffErrorKindString[accountKeyErrorKind],
					Msg:     fmt.Sprintf("failed to get account public key v3 at key index %d: %s", keyIndex, err),
				})
			continue
		}

		keyV4, err := accountsV4.getAccountPublicKey(flow.Address(akd.address), keyIndex)
		if err != nil {
			akd.reportWriter.Write(
				accountKeyDiffError{
					Address: akd.address.Hex(),
					Kind:    accountKeyDiffErrorKindString[accountKeyErrorKind],
					Msg:     fmt.Sprintf("failed to get account public key v4 at key index %d: %s", keyIndex, err),
				})
			continue
		}

		err = equal(keyV3, keyV4)
		if err != nil {
			encodedKeyV3, _ := json.Marshal(keyV3)
			encodedKeyV4, _ := json.Marshal(keyV4)

			akd.reportWriter.Write(
				accountKeyDiffProblem{
					Address:  akd.address.Hex(),
					KeyIndex: keyIndex,
					Kind:     accountKeyDiffKindString[accountKeyDiff],
					Msg:      fmt.Sprintf("v3: %s, v4 %s: %s", encodedKeyV3, encodedKeyV4, err.Error()),
				})
		}
	}
}

type accountsWithAccountKeysV3 struct {
	a *environment.StatefulAccounts
}

func newAccountsWithAccountKeyV3(regs registers.Registers) *accountsWithAccountKeysV3 {
	return &accountsWithAccountKeysV3{
		a: newStatefulAccounts(regs),
	}
}

func (akv3 *accountsWithAccountKeysV3) getAccountPublicKeyCount(address flow.Address) (uint32, error) {
	id := flow.AccountStatusRegisterID(address)

	statusBytes, err := akv3.a.GetValue(id)
	if err != nil {
		return 0, fmt.Errorf("failed to load account status for account %s: %w", address, err)
	}
	if len(statusBytes) == 0 {
		return 0, fmt.Errorf("account status register is empty for account %s", address)
	}

	as, err := environment.AccountStatusFromBytes(statusBytes)
	if err != nil {
		return 0, fmt.Errorf("failed to create account status from bytes for account %s: %w", address, err)
	}

	return as.AccountPublicKeyCount(), nil
}

func (akv3 *accountsWithAccountKeysV3) getAccountPublicKey(address flow.Address, keyIndex uint32) (flow.AccountPublicKey, error) {
	id := flow.RegisterID{
		Owner: string(address.Bytes()),
		Key:   fmt.Sprintf(legacyAccountPublicKeyRegisterKeyPattern, keyIndex),
	}

	publicKeyBytes, err := akv3.a.GetValue(id)
	if err != nil {
		return flow.AccountPublicKey{}, fmt.Errorf("failed to load account public key %d for account %s: %w", keyIndex, address, err)
	}
	if len(publicKeyBytes) == 0 {
		return flow.AccountPublicKey{}, fmt.Errorf("account public key %d is empty for account %s", keyIndex, address)
	}

	key, err := flow.DecodeAccountPublicKey(publicKeyBytes, keyIndex)
	if err != nil {
		return flow.AccountPublicKey{}, fmt.Errorf("failed to decode account public key %d for account %s", keyIndex, address)
	}

	return key, nil
}

type accountsWithAccountKeysV4 struct {
	a *environment.StatefulAccounts
}

func newAccountsWithAccountKeyV4(regs registers.Registers) *accountsWithAccountKeysV4 {
	return &accountsWithAccountKeysV4{
		a: newStatefulAccounts(regs),
	}
}

func (akv4 *accountsWithAccountKeysV4) getAccountPublicKeyCount(address flow.Address) (uint32, error) {
	return akv4.a.GetAccountPublicKeyCount(address)
}

func (akv4 *accountsWithAccountKeysV4) getAccountPublicKey(address flow.Address, keyIndex uint32) (flow.AccountPublicKey, error) {
	return akv4.a.GetAccountPublicKey(address, keyIndex)
}

func newStatefulAccounts(
	regs registers.Registers,
) *environment.StatefulAccounts {
	// Create a new transaction state with a dummy hasher
	// because we do not need spock proofs for migrations.
	transactionState := state.NewTransactionStateFromExecutionState(
		state.NewSpockExecutionStateWithSpockStateHasher(
			registers.StorageSnapshot{
				Registers: regs,
			},
			state.DefaultParameters(),
			func() hash.Hasher {
				return dummyHasher{}
			},
		),
	)
	return environment.NewAccounts(transactionState)
}

func equal(keyV3, keyV4 flow.AccountPublicKey) error {
	if keyV3.Index != keyV4.Index {
		return fmt.Errorf("account public key index differ: v3 %v, v4 %v", keyV3.Index, keyV4.Index)
	}

	if !keyV3.PublicKey.Equals(keyV4.PublicKey) {
		return fmt.Errorf("account public key differ: v3 %v, v4 %v", keyV3.PublicKey, keyV4.PublicKey)
	}

	if keyV3.SignAlgo != keyV4.SignAlgo {
		return fmt.Errorf("account public key sign algo differ: v3 %v, v4 %v", keyV3.SignAlgo, keyV4.SignAlgo)
	}

	if keyV3.HashAlgo != keyV4.HashAlgo {
		return fmt.Errorf("account public key hash algo differ: v3 %v, v4 %v", keyV3.HashAlgo, keyV4.HashAlgo)
	}

	if keyV3.SeqNumber != keyV4.SeqNumber {
		return fmt.Errorf("account public key sequence number differ: v3 %v, v4 %v", keyV3.SeqNumber, keyV4.SeqNumber)
	}

	if keyV3.Weight != keyV4.Weight {
		return fmt.Errorf("account public key weight differ: v3 %v, v4 %v", keyV3.Weight, keyV4.Weight)
	}

	if keyV3.Revoked != keyV4.Revoked {
		return fmt.Errorf("account public key revoked status differ: v3 %v, v4 %v", keyV3.Revoked, keyV4.Revoked)
	}

	return nil
}
