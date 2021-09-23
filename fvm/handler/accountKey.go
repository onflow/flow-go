package handler

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/sema"

	fgcrypto "github.com/onflow/flow-go/crypto"
	fghash "github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

// AccountKeyHandler handles all interaction
// with account keys such as get/set/revoke
type AccountKeyHandler struct {
	accounts state.Accounts
}

//  NewAccountPublicKey construct an account public key given a runtime public key.
func NewAccountPublicKey(publicKey *runtime.PublicKey,
	hashAlgo sema.HashAlgorithm,
	keyIndex int,
	weight int,
) (*flow.AccountPublicKey, error) {

	var err error
	signAlgorithm := crypto.RuntimeToCryptoSigningAlgorithm(publicKey.SignAlgo)
	if signAlgorithm != fgcrypto.ECDSAP256 && signAlgorithm != fgcrypto.ECDSASecp256k1 {
		err = errors.NewValueErrorf(string(publicKey.SignAlgo), "signature algorithm type not supported")
		return nil, fmt.Errorf("adding account key failed: %w", err)
	}

	hashAlgorithm := crypto.RuntimeToCryptoHashingAlgorithm(hashAlgo)
	if hashAlgorithm != fghash.SHA2_256 && hashAlgorithm != fghash.SHA3_256 {
		err = errors.NewValueErrorf(string(hashAlgo), "hashing algorithm type not supported")
		return nil, fmt.Errorf("adding account key failed: %w", err)
	}

	decodedPublicKey, err := fgcrypto.DecodePublicKey(signAlgorithm, publicKey.PublicKey)
	if err != nil {
		err = errors.NewValueErrorf(string(publicKey.PublicKey), "cannot decode public key: %w", err)
		return nil, fmt.Errorf("adding account key failed: %w", err)
	}

	return &flow.AccountPublicKey{
		Index:     keyIndex,
		PublicKey: decodedPublicKey,
		SignAlgo:  signAlgorithm,
		HashAlgo:  hashAlgorithm,
		SeqNumber: 0,
		Weight:    weight,
		Revoked:   false,
	}, nil
}

func NewAccountKeyHandler(accounts *state.AccountsState) *AccountKeyHandler {
	return &AccountKeyHandler{
		accounts: accounts,
	}
}

// AddAccountKey adds a public key to an existing account.
//
// This function returns an error if the specified account does not exist or
// if the key insertion fails.
func (h *AccountKeyHandler) AddAccountKey(address runtime.Address,
	publicKey *runtime.PublicKey,
	hashAlgo runtime.HashAlgorithm,
	weight int,
) (
	*runtime.AccountKey,
	error,
) {
	accountAddress := flow.Address(address)

	ok, err := h.accounts.Exists(accountAddress)
	if err != nil {
		return nil, fmt.Errorf("adding account key failed: %w", err)
	}
	if !ok {
		issue := errors.NewAccountNotFoundError(accountAddress)
		return nil, fmt.Errorf("adding account key failed: %w", issue)
	}

	keyIndex, err := h.accounts.GetPublicKeyCount(accountAddress)
	if err != nil {
		return nil, fmt.Errorf("adding account key failed: %w", err)
	}

	accountPublicKey, err := NewAccountPublicKey(publicKey, hashAlgo, int(keyIndex), weight)
	if err != nil {
		return nil, fmt.Errorf("adding account key failed: %w", err)
	}

	err = h.accounts.AppendPublicKey(accountAddress, *accountPublicKey)
	if err != nil {
		return nil, fmt.Errorf("adding account key failed: %w", err)
	}

	return &runtime.AccountKey{
		KeyIndex:  accountPublicKey.Index,
		PublicKey: publicKey,
		HashAlgo:  hashAlgo,
		Weight:    accountPublicKey.Weight,
		IsRevoked: accountPublicKey.Revoked,
	}, nil
}

// RevokeAccountKey revokes a public key by index from an existing account,
// and returns the revoked key.
//
// This function returns a nil key with no errors, if a key doesn't exist at the given index.
// An error is returned if the specified account does not exist, the provided index is not valid,
// or if the key revoking fails.
// TODO (ramtin) do we have to return runtime.AccountKey for this method or can be separated into another method
func (h *AccountKeyHandler) RevokeAccountKey(address runtime.Address, keyIndex int) (*runtime.AccountKey, error) {
	accountAddress := flow.Address(address)

	ok, err := h.accounts.Exists(accountAddress)
	if err != nil {
		return nil, fmt.Errorf("revoking account key failed: %w", err)
	}

	if !ok {
		issue := errors.NewAccountNotFoundError(accountAddress)
		return nil, fmt.Errorf("revoking account key failed: %w", issue)
	}

	// Don't return an error for invalid key indices
	if keyIndex < 0 {
		return nil, nil
	}

	var publicKey flow.AccountPublicKey
	publicKey, err = h.accounts.GetPublicKey(accountAddress, uint64(keyIndex))
	if err != nil {
		// If a key is not found at a given index, then return a nil key with no errors.
		// This is to be inline with the Cadence runtime. Otherwise Cadence runtime cannot
		// distinguish between a 'key not found error' vs other internal errors.
		if errors.IsAccountAccountPublicKeyNotFoundError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("revoking account key failed: %w", err)
	}

	// mark this key as revoked
	publicKey.Revoked = true

	_, err = h.accounts.SetPublicKey(accountAddress, uint64(keyIndex), publicKey)
	if err != nil {
		return nil, fmt.Errorf("revoking account key failed: %w", err)
	}

	// Prepare account key to return
	signAlgo := crypto.CryptoToRuntimeSigningAlgorithm(publicKey.SignAlgo)
	if signAlgo == runtime.SignatureAlgorithmUnknown {
		err = errors.NewValueErrorf(publicKey.SignAlgo.String(), "signature algorithm type not found")
		return nil, fmt.Errorf("revoking account key failed: %w", err)
	}

	hashAlgo := crypto.CryptoToRuntimeHashingAlgorithm(publicKey.HashAlgo)
	if hashAlgo == runtime.HashAlgorithmUnknown {
		err = errors.NewValueErrorf(publicKey.HashAlgo.String(), "hashing algorithm type not found")
		return nil, fmt.Errorf("revoking account key failed: %w", err)
	}

	return &runtime.AccountKey{
		KeyIndex: publicKey.Index,
		PublicKey: &runtime.PublicKey{
			PublicKey: publicKey.PublicKey.Encode(),
			SignAlgo:  signAlgo,
		},
		HashAlgo:  hashAlgo,
		Weight:    publicKey.Weight,
		IsRevoked: publicKey.Revoked,
	}, nil
}

// GetAccountKey retrieves a public key by index from an existing account.
//
// This function returns a nil key with no errors, if a key doesn't exist at the given index.
// An error is returned if the specified account does not exist, the provided index is not valid,
// or if the key retrieval fails.
func (h *AccountKeyHandler) GetAccountKey(address runtime.Address, keyIndex int) (*runtime.AccountKey, error) {
	accountAddress := flow.Address(address)

	ok, err := h.accounts.Exists(accountAddress)
	if err != nil {
		return nil, fmt.Errorf("getting account key failed: %w", err)
	}

	if !ok {
		issue := errors.NewAccountNotFoundError(accountAddress)
		return nil, fmt.Errorf("getting account key failed: %w", issue)
	}

	// Don't return an error for invalid key indices
	if keyIndex < 0 {
		return nil, nil
	}

	var publicKey flow.AccountPublicKey
	publicKey, err = h.accounts.GetPublicKey(accountAddress, uint64(keyIndex))
	if err != nil {
		// If a key is not found at a given index, then return a nil key with no errors.
		// This is to be inline with the Cadence runtime. Otherwise Cadence runtime cannot
		// distinguish between a 'key not found error' vs other internal errors.
		if errors.IsAccountAccountPublicKeyNotFoundError(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("getting account key failed: %w", err)
	}

	// Prepare the account key to return

	signAlgo := crypto.CryptoToRuntimeSigningAlgorithm(publicKey.SignAlgo)
	if signAlgo == runtime.SignatureAlgorithmUnknown {
		err = errors.NewValueErrorf(publicKey.SignAlgo.String(), "signature algorithm type not found")
		return nil, fmt.Errorf("getting account key failed: %w", err)
	}

	hashAlgo := crypto.CryptoToRuntimeHashingAlgorithm(publicKey.HashAlgo)
	if hashAlgo == runtime.HashAlgorithmUnknown {
		err = errors.NewValueErrorf(publicKey.HashAlgo.String(), "hashing algorithm type not found")
		return nil, fmt.Errorf("getting account key failed: %w", err)
	}

	return &runtime.AccountKey{
		KeyIndex: publicKey.Index,
		PublicKey: &runtime.PublicKey{
			PublicKey: publicKey.PublicKey.Encode(),
			SignAlgo:  signAlgo,
		},
		HashAlgo:  hashAlgo,
		Weight:    publicKey.Weight,
		IsRevoked: publicKey.Revoked,
	}, nil
}

// AddEncodedAccountKey adds an encoded public key to an existing account.
//
// This function returns an error if the specified account does not exist or
// if the key insertion fails.
func (e *AccountKeyHandler) AddEncodedAccountKey(address runtime.Address, encodedPublicKey []byte) (err error) {
	accountAddress := flow.Address(address)

	ok, err := e.accounts.Exists(accountAddress)
	if err != nil {
		return fmt.Errorf("adding encoded account key failed: %w", err)
	}

	if !ok {
		return errors.NewAccountNotFoundError(accountAddress)
	}

	var publicKey flow.AccountPublicKey

	publicKey, err = flow.DecodeRuntimeAccountPublicKey(encodedPublicKey, 0)
	if err != nil {
		hexEncodedPublicKey := hex.EncodeToString(encodedPublicKey)
		err = errors.NewValueErrorf(hexEncodedPublicKey, "invalid encoded public key value: %w", err)
		return fmt.Errorf("adding encoded account key failed: %w", err)
	}

	err = e.accounts.AppendPublicKey(accountAddress, publicKey)
	if err != nil {
		return fmt.Errorf("adding encoded account key failed: %w", err)
	}

	return nil
}

// RemoveAccountKey revokes a public key by index from an existing account.
//
// This function returns an error if the specified account does not exist, the
// provided key is invalid, or if key revoking fails.
func (e *AccountKeyHandler) RemoveAccountKey(address runtime.Address, keyIndex int) (encodedPublicKey []byte, err error) {
	accountAddress := flow.Address(address)

	ok, err := e.accounts.Exists(accountAddress)
	if err != nil {
		return nil, fmt.Errorf("remove account key failed: %w", err)
	}

	if !ok {
		issue := errors.NewAccountNotFoundError(accountAddress)
		return nil, fmt.Errorf("remove account key failed: %w", issue)
	}

	if keyIndex < 0 {
		err = errors.NewValueErrorf(fmt.Sprint(keyIndex), "key index must be positive")
		return nil, fmt.Errorf("remove account key failed: %w", err)
	}

	var publicKey flow.AccountPublicKey
	publicKey, err = e.accounts.GetPublicKey(accountAddress, uint64(keyIndex))
	if err != nil {
		return nil, fmt.Errorf("remove account key failed: %w", err)
	}

	// mark this key as revoked
	publicKey.Revoked = true

	encodedPublicKey, err = e.accounts.SetPublicKey(accountAddress, uint64(keyIndex), publicKey)
	if err != nil {
		return nil, fmt.Errorf("remove account key failed: %w", err)
	}

	return encodedPublicKey, nil
}
