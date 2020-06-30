package fvm

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/dapperlabs/flow-go/model/flow"
)

const AccountKeyWeightThreshold = 1000

const (
	keyAddressState   = "account_address_state"
	keyExists         = "exists"
	keyCode           = "code"
	keyPublicKeyCount = "public_key_count"
)

func keyPublicKey(index uint64) string {
	return fmt.Sprintf("public_key_%d", index)
}

func getAccount(
	vm *VirtualMachine,
	ctx Context,
	ledger Ledger,
	chain flow.Chain,
	address flow.Address,
) (*flow.Account, error) {
	var ok bool
	var err error

	ok, err = accountExists(ledger, address)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, ErrAccountNotFound
	}

	var code []byte
	code, err = getAccountCode(ledger, address)
	if err != nil {
		return nil, err
	}

	var publicKeys []flow.AccountPublicKey
	publicKeys, err = getAccountPublicKeys(ledger, address)
	if err != nil {
		return nil, err
	}

	var result *InvocationResult
	result, err = vm.Invoke(
		ctx,
		getFlowTokenBalanceScript(address, chain.ServiceAddress()),
		ledger,
	)
	if err != nil {
		return nil, err
	}

	var balance uint64

	// TODO: Figure out how to handle this error. Currently if a runtime error occurs, balance will be 0.
	// 1. An error will occur if user has removed their FlowToken.Vault -- should this be allowed?
	// 2. Any other error indicates a bug in our implementation. How can we reliably check the Cadence error?
	if result.Error == nil {
		balance = result.Value.ToGoValue().(uint64)
	}

	return &flow.Account{
		Address: address,
		Balance: balance,
		Code:    code,
		Keys:    publicKeys,
	}, nil
}

func getAccountCode(ledger Ledger, address flow.Address) ([]byte, error) {
	code, err := ledger.Get(fullKeyHash(string(address.Bytes()), string(address.Bytes()), keyCode))
	if err != nil {
		return nil, newLedgerGetError(keyCode, address, err)
	}

	return code, nil
}

func accountExists(ledger Ledger, address flow.Address) (bool, error) {
	exists, err := ledger.Get(fullKeyHash(string(address.Bytes()), "", keyExists))
	if err != nil {
		return false, newLedgerGetError(keyExists, address, err)
	}

	if len(exists) != 0 {
		return true, nil
	}

	return false, nil
}

func createAccount(ledger Ledger, chain flow.Chain, publicKeys []flow.AccountPublicKey) (flow.Address, error) {
	addressState, err := getAddressState(ledger, chain)
	if err != nil {
		return flow.EmptyAddress, err
	}

	// generate the new account address
	var newAddress flow.Address
	newAddress, err = addressState.NextAddress()

	if err != nil {
		return flow.EmptyAddress, err
	}

	err = createAccountWithAddress(ledger, newAddress, publicKeys)
	if err != nil {
		return flow.Address{}, err
	}

	// update the address state
	setAddressState(ledger, addressState)

	return newAddress, nil
}

func createAccountWithAddress(
	ledger Ledger,
	address flow.Address,
	publicKeys []flow.AccountPublicKey,
) error {
	// mark that this account exists
	ledger.Set(fullKeyHash(string(address.Bytes()), "", keyExists), []byte{1})

	ledger.Set(fullKeyHash(string(address.Bytes()), string(address.Bytes()), keyCode), nil)

	err := setAccountPublicKeys(ledger, address, publicKeys)
	if err != nil {
		return err
	}

	return nil
}

func getAccountPublicKeys(ledger Ledger, address flow.Address) (publicKeys []flow.AccountPublicKey, err error) {
	var countBytes []byte
	countBytes, err = ledger.Get(
		fullKeyHash(string(address.Bytes()), string(address.Bytes()), keyPublicKeyCount),
	)
	if err != nil {
		return nil, newLedgerGetError(keyPublicKeyCount, address, err)
	}

	if countBytes == nil {
		return nil, fmt.Errorf("public key count not set on account %s", address)
	}

	countInt := new(big.Int).SetBytes(countBytes)
	if !countInt.IsUint64() {
		return nil, fmt.Errorf(
			"retrieved public key account count bytes (hex-encoded): %x do not represent valid uint64",
			countBytes,
		)
	}
	count := countInt.Uint64()

	publicKeys = make([]flow.AccountPublicKey, count)

	for i := uint64(0); i < count; i++ {
		publicKey, err := getAccountPublicKey(ledger, address, i)
		if err != nil {
			return nil, err
		}

		publicKeys[i] = publicKey
	}

	return publicKeys, nil
}

func getAccountPublicKey(ledger Ledger, address flow.Address, keyIndex uint64) (flow.AccountPublicKey, error) {
	publicKey, err := ledger.Get(
		fullKeyHash(string(address.Bytes()), string(address.Bytes()), keyPublicKey(keyIndex)),
	)
	if err != nil {
		return flow.AccountPublicKey{}, newLedgerGetError(keyPublicKey(keyIndex), address, err)
	}

	if publicKey == nil {
		return flow.AccountPublicKey{}, ErrAccountPublicKeyNotFound
	}

	decodedPublicKey, err := flow.DecodeAccountPublicKey(publicKey)
	if err != nil {
		return flow.AccountPublicKey{}, fmt.Errorf("failed to decode public key: %w", err)
	}

	return decodedPublicKey, nil
}

func setAccountPublicKeys(ledger Ledger, address flow.Address, publicKeys []flow.AccountPublicKey) error {

	var existingCount uint64

	countBytes, err := ledger.Get(
		fullKeyHash(string(address.Bytes()), string(address.Bytes()), keyPublicKeyCount),
	)
	if err != nil {
		return newLedgerGetError(keyPublicKeyCount, address, err)
	}

	if countBytes != nil {
		countInt := new(big.Int).SetBytes(countBytes)
		if !countInt.IsUint64() {
			return fmt.Errorf(
				"retrieved public key account bytes (hex): %x do not represent valid uint64",
				countBytes,
			)
		}
		existingCount = countInt.Uint64()
	} else {
		existingCount = 0
	}

	newCount := uint64(len(publicKeys)) // len returns int and this won't exceed uint64
	newKeyCount := new(big.Int).SetUint64(newCount)

	ledger.Set(
		fullKeyHash(string(address.Bytes()), string(address.Bytes()), keyPublicKeyCount),
		newKeyCount.Bytes(),
	)

	for i, publicKey := range publicKeys {
		err = publicKey.Validate()
		if err != nil {
			return fmt.Errorf("invalid public key: %w", err)
		}

		// asserted length of publicKeys so index should always fit into uint64
		_, err = setAccountPublicKey(ledger, address, uint64(i), publicKey)
		if err != nil {
			return err
		}
	}

	// delete leftover keys
	for i := newCount; i < existingCount; i++ {
		ledger.Delete(fullKeyHash(string(address.Bytes()), string(address.Bytes()), keyPublicKey(i)))
	}

	return nil
}

func setAccountPublicKey(
	ledger Ledger,
	address flow.Address,
	keyIndex uint64,
	publicKey flow.AccountPublicKey,
) (encodedPublicKey []byte, err error) {
	encodedPublicKey, err = flow.EncodeAccountPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encode public key: %w", err)
	}

	ledger.Set(
		fullKeyHash(string(address.Bytes()), string(address.Bytes()), keyPublicKey(keyIndex)),
		encodedPublicKey,
	)

	return encodedPublicKey, nil
}

func setAccountCode(ledger Ledger, address flow.Address, code []byte) error {
	ok, err := accountExists(ledger, address)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("account with address %s does not exist", address)
	}

	key := fullKeyHash(string(address.Bytes()), string(address.Bytes()), keyCode)

	var prevCode []byte
	prevCode, err = ledger.Get(key)
	if err != nil {
		return fmt.Errorf("cannot retreive previous code: %w", err)
	}

	// skip updating if the new code equals the old
	if bytes.Equal(prevCode, code) {
		return nil
	}

	ledger.Set(key, code)

	return nil
}

func getAddressState(ledger Ledger, chain flow.Chain) (flow.AddressGenerator, error) {
	stateBytes, err := ledger.Get(fullKeyHash("", "", keyAddressState))
	if err != nil {
		return nil, err
	}

	return chain.BytesToAddressState(stateBytes), nil
}

func setAddressState(ledger Ledger, state flow.AddressGenerator) {
	stateBytes := state.Bytes()
	ledger.Set(fullKeyHash("", "", keyAddressState), stateBytes)
}

const initFlowTokenTransactionTemplate = `
import FlowServiceAccount from 0x%s

transaction {
  prepare(account: AuthAccount) {
    FlowServiceAccount.initDefaultToken(account)
  }
}
`

const getFlowTokenBalanceScriptTemplate = `
import FlowServiceAccount from 0x%s

pub fun main(): UFix64 {
  let acct = getAccount(0x%s)
  return FlowServiceAccount.defaultTokenBalance(acct)
}
`

func initFlowTokenTransaction(accountAddress, serviceAddress flow.Address) InvokableTransaction {
	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(initFlowTokenTransactionTemplate, serviceAddress))).
			AddAuthorizer(accountAddress),
	)
}

func getFlowTokenBalanceScript(accountAddress, serviceAddress flow.Address) InvokableScript {
	return Script([]byte(fmt.Sprintf(getFlowTokenBalanceScriptTemplate, serviceAddress, accountAddress)))
}

func newLedgerGetError(key string, address flow.Address, err error) error {
	return fmt.Errorf("failed to read key %s on account %s: %w", key, address, err)
}
