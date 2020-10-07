package state

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"sort"

	"github.com/onflow/flow-go/model/encoding/rlp"
	"github.com/onflow/flow-go/model/flow"
)

const (
	keyExists         = "exists"
	keyCode           = "code"
	keyContracts      = "contracts"
	keyPublicKeyCount = "public_key_count"
)

var (
	ErrAccountNotFound          = errors.New("account not found")
	ErrAccountPublicKeyNotFound = errors.New("account public key not found")
)

func keyPublicKey(index uint64) string {
	return fmt.Sprintf("public_key_%d", index)
}

type Accounts struct {
	ledger Ledger
	*addresses
}

// ContractsList should always be sorted. To ensure this, don't sort while reading it from storage, but sort it while adding elements
type ContractsList []string

func (l ContractsList) Has(contract string) bool {
	i := sort.SearchStrings(l, contract)
	return i != len(l) && l[i] == contract
}
func (l *ContractsList) add(contract string) {
	i := sort.SearchStrings(*l, contract)
	if i != len(*l) && (*l)[i] == contract {
		// list already contains element
		return
	}
	*l = append(*l, "")
	copy((*l)[i+1:], (*l)[i:])
	(*l)[i] = contract
}
func (l *ContractsList) remove(contract string) {
	i := sort.SearchStrings(*l, contract)
	if i == len(*l) || (*l)[i] != contract {
		// list doesnt contain the element element
		return
	}
	*l = append((*l)[:i], (*l)[i+1:]...)
}

func NewAccounts(ledger Ledger, chain flow.Chain) *Accounts {
	addresses := newAddresses(ledger, chain)

	return &Accounts{
		ledger:    ledger,
		addresses: addresses,
	}
}

func (a *Accounts) Get(address flow.Address) (*flow.Account, error) {
	var ok bool
	var err error

	ok, err = a.Exists(address)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, ErrAccountNotFound
	}
	contracts := make(map[string][]byte)
	contractNames, err := a.GetContracts(address)

	if err != nil {
		return nil, err
	}
	for _, name := range contractNames {
		code, err := a.getCode(name, address)
		if err != nil {
			return nil, err
		}
		contracts[name] = code
	}

	var publicKeys []flow.AccountPublicKey
	publicKeys, err = a.GetPublicKeys(address)
	if err != nil {
		return nil, err
	}

	return &flow.Account{
		Address:   address,
		Code:      nil,
		Keys:      publicKeys,
		Contracts: contracts,
	}, nil
}

func (a *Accounts) Exists(address flow.Address) (bool, error) {
	exists, err := a.ledger.Get(string(address.Bytes()), "", keyExists)
	if err != nil {
		return false, newLedgerGetError(keyExists, address, err)
	}

	if len(exists) != 0 {
		return true, nil
	}

	return false, nil
}

func (a *Accounts) Create(publicKeys []flow.AccountPublicKey) (flow.Address, error) {
	addressState, err := a.addresses.GetAddressGeneratorState()
	if err != nil {
		return flow.EmptyAddress, err
	}

	// generate the new account address
	var newAddress flow.Address
	newAddress, err = addressState.NextAddress()
	if err != nil {
		return flow.EmptyAddress, err
	}

	// mark that this account exists
	a.ledger.Set(string(newAddress.Bytes()), "", keyExists, []byte{1})

	err = a.SetAllPublicKeys(newAddress, publicKeys)
	if err != nil {
		return flow.EmptyAddress, err
	}

	// update the address state
	a.addresses.SetAddressGeneratorState(addressState)

	return newAddress, nil
}

func (a *Accounts) GetPublicKey(address flow.Address, keyIndex uint64) (flow.AccountPublicKey, error) {
	publicKey, err := a.ledger.Get(
		string(address.Bytes()), string(address.Bytes()), keyPublicKey(keyIndex),
	)
	if err != nil {
		return flow.AccountPublicKey{}, newLedgerGetError(keyPublicKey(keyIndex), address, err)
	}

	if publicKey == nil {
		return flow.AccountPublicKey{}, ErrAccountPublicKeyNotFound
	}

	decodedPublicKey, err := flow.DecodeAccountPublicKey(publicKey, keyIndex)
	if err != nil {
		return flow.AccountPublicKey{}, fmt.Errorf("failed to decode public key: %w", err)
	}

	return decodedPublicKey, nil
}

func (a *Accounts) GetPublicKeyCount(address flow.Address) (uint64, error) {
	countBytes, err := a.ledger.Get(
		string(address.Bytes()), string(address.Bytes()), keyPublicKeyCount,
	)
	if err != nil {
		return 0, newLedgerGetError(keyPublicKeyCount, address, err)
	}

	if countBytes == nil {
		return 0, nil
	}

	countInt := new(big.Int).SetBytes(countBytes)
	if !countInt.IsUint64() {
		return 0, fmt.Errorf(
			"retrieved public key account count bytes (hex-encoded): %x does not represent valid uint64",
			countBytes,
		)
	}

	return countInt.Uint64(), nil
}

func (a *Accounts) SetPublicKeyCount(address flow.Address, count uint64) {
	newCount := new(big.Int).SetUint64(count)

	a.ledger.Set(
		string(address.Bytes()), string(address.Bytes()), keyPublicKeyCount,
		newCount.Bytes(),
	)
}

func (a *Accounts) GetPublicKeys(address flow.Address) (publicKeys []flow.AccountPublicKey, err error) {
	var countBytes []byte
	countBytes, err = a.ledger.Get(
		string(address.Bytes()), string(address.Bytes()), keyPublicKeyCount,
	)
	if err != nil {
		return nil, newLedgerGetError(keyPublicKeyCount, address, err)
	}

	var count uint64

	if countBytes == nil {
		count = 0
	} else {
		countInt := new(big.Int).SetBytes(countBytes)
		if !countInt.IsUint64() {
			return nil, fmt.Errorf(
				"retrieved public key account count bytes (hex-encoded): %x do not represent valid uint64",
				countBytes,
			)
		}
		count = countInt.Uint64()
	}

	publicKeys = make([]flow.AccountPublicKey, count)

	for i := uint64(0); i < count; i++ {
		publicKey, err := a.GetPublicKey(address, i)
		if err != nil {
			return nil, err
		}

		publicKeys[i] = publicKey
	}

	return publicKeys, nil
}

func (a *Accounts) SetPublicKey(
	address flow.Address,
	keyIndex uint64,
	publicKey flow.AccountPublicKey,
) (encodedPublicKey []byte, err error) {
	err = publicKey.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid public key: %w", err)
	}

	encodedPublicKey, err = flow.EncodeAccountPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encode public key: %w", err)
	}

	a.ledger.Set(
		string(address.Bytes()), string(address.Bytes()), keyPublicKey(keyIndex),
		encodedPublicKey,
	)

	return encodedPublicKey, nil
}

func (a *Accounts) SetAllPublicKeys(address flow.Address, publicKeys []flow.AccountPublicKey) error {

	for i, publicKey := range publicKeys {
		_, err := a.SetPublicKey(address, uint64(i), publicKey)
		if err != nil {
			return err
		}
	}

	count := uint64(len(publicKeys)) // len returns int and this will not exceed uint64

	a.SetPublicKeyCount(address, count)

	return nil
}

func (a *Accounts) AppendPublicKey(address flow.Address, publicKey flow.AccountPublicKey) error {
	count, err := a.GetPublicKeyCount(address)
	if err != nil {
		return err
	}

	_, err = a.SetPublicKey(address, count, publicKey)
	if err != nil {
		return err
	}

	a.SetPublicKeyCount(address, count+1)

	return nil
}

func (a *Accounts) contractKey(contractName string) string {
	return fmt.Sprintf("%s.%s", keyCode, contractName)
}

func (a *Accounts) getCode(name string, address flow.Address) ([]byte, error) {

	code, err := a.ledger.Get(string(address.Bytes()),
		string(address.Bytes()),
		a.contractKey(name))
	if err != nil {
		return nil, newLedgerGetError(name, address, err)
	}

	return code, nil
}

func (a *Accounts) setCode(name string, address flow.Address, code []byte) error {
	ok, err := a.Exists(address)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("account with address %s does not exist", address)
	}

	var prevCode []byte
	prevCode, err = a.ledger.Get(string(address.Bytes()), string(address.Bytes()), a.contractKey(name))
	if err != nil {
		return fmt.Errorf("cannot retreive previous code: %w", err)
	}

	// skip updating if the new code equals the old
	if bytes.Equal(prevCode, code) {
		return nil
	}

	a.ledger.Set(string(address.Bytes()), string(address.Bytes()), a.contractKey(name), code)

	return nil
}

func newLedgerGetError(key string, address flow.Address, err error) error {
	return fmt.Errorf("failed to read key %s on account %s: %w", key, address, err)
}

func (a *Accounts) setContracts(contracts ContractsList, address flow.Address) error {
	ok, err := a.Exists(address)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("account with address %s does not exist", address)
	}

	newContracts, err := rlp.NewEncoder().Encode(contracts)
	if err != nil {
		return fmt.Errorf("cannot serialize contract list")
	}

	var prevContracts []byte
	prevContracts, err = a.ledger.Get(string(address.Bytes()), string(address.Bytes()), keyContracts)
	if err != nil {
		return fmt.Errorf("cannot retreive current contracts: %w", err)
	}

	// skip updating if the new contracts equal the old
	if bytes.Equal(prevContracts, newContracts) {
		return nil
	}

	a.ledger.Set(string(address.Bytes()), string(address.Bytes()), keyContracts, newContracts)

	return nil
}

func (a *Accounts) TouchContract(name string, address flow.Address) {
	contracts, err := a.GetContracts(address)
	if err != nil {
		panic(err)
	}
	if contracts.Has(name) {
		a.ledger.Touch(string(address.Bytes()),
			string(address.Bytes()),
			a.contractKey(name))
		a.ledger.Touch(string(address.Bytes()),
			string(address.Bytes()),
			keyExists)
	}
}

func (a *Accounts) GetContracts(address flow.Address) (ContractsList, error) {
	encContractList, err := a.ledger.Get(string(address.Bytes()), string(address.Bytes()), keyContracts)
	if err != nil {
		return nil, fmt.Errorf("cannot get deployed contract list: %w", err)
	}
	identifiers := make([]string, 0)
	if len(encContractList) > 0 {
		err = rlp.NewEncoder().Decode(encContractList, &identifiers)
		if err != nil {
			return nil, fmt.Errorf("cannot decode deployed contract list %x: %w", encContractList, err)
		}
	}
	return identifiers, nil
}

func (a *Accounts) GetContract(name string, address flow.Address) ([]byte, error) {
	contracts, err := a.GetContracts(address)
	if err != nil {
		return nil, err
	}
	if !contracts.Has(name) {
		return nil, nil
	}
	return a.getCode(name, address)
}

func (a *Accounts) SetContract(name string, address flow.Address, code []byte) error {
	contracts, err := a.GetContracts(address)
	if err != nil {
		return err
	}
	err = a.setCode(name, address, code)
	if err != nil {
		return err
	}
	if !contracts.Has(name) {
		contracts.add(name)
		return a.setContracts(contracts, address)
	}
	return nil
}

func (a *Accounts) DeleteContract(name string, address flow.Address) error {
	contracts, err := a.GetContracts(address)
	if err != nil {
		return err
	}
	if !contracts.Has(name) {
		return nil
	}
	err = a.setCode(name, address, nil)
	if err != nil {
		return err
	}
	contracts.remove(name)
	return a.setContracts(contracts, address)
}
