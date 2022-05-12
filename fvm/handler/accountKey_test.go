package handler

import (
	errors2 "errors"
	"fmt"
	"testing"
	"unicode/utf8"

	"github.com/fxamacker/cbor/v2"
	"github.com/onflow/atree"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/errors"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/flow"
)

func TestAddEncodedAccountKey_error_handling_produces_valid_utf8(t *testing.T) {

	akh := &AccountKeyHandler{accounts: FakeAccounts{}}

	address := cadence.BytesToAddress([]byte{1, 2, 3, 4})

	// emulate encoded public key (which comes as a user input)
	// containing bytes which are invalid UTF8

	invalidEncodedKey := make([]byte, 64)
	invalidUTF8 := []byte{0xc3, 0x28}
	copy(invalidUTF8, invalidEncodedKey)
	publicKey := FakePublicKey{data: invalidEncodedKey}

	accountPublicKey := flow.AccountPublicKey{
		Index:     1,
		PublicKey: publicKey,
		SignAlgo:  crypto.ECDSASecp256k1,
		HashAlgo:  hash.SHA3_256,
		SeqNumber: 0,
		Weight:    1000,
		Revoked:   false,
	}

	encodedPublicKey, err := flow.EncodeRuntimeAccountPublicKey(accountPublicKey)
	require.NoError(t, err)

	err = akh.AddEncodedAccountKey(runtime.Address(address), encodedPublicKey)
	require.Error(t, err)

	err = errors2.Unwrap(err)
	require.IsType(t, &errors.ValueError{}, err)

	errorString := err.Error()
	assert.True(t, utf8.ValidString(errorString))

	// check if they can encoded and decoded using CBOR
	marshalledBytes, err := cbor.Marshal(errorString)
	require.NoError(t, err)

	var unmarshalledString string

	err = cbor.Unmarshal(marshalledBytes, &unmarshalledString)
	require.NoError(t, err)

	require.Equal(t, errorString, unmarshalledString)
}

func TestNewAccountKey_error_handling_produces_valid_utf8_and_sign_algo(t *testing.T) {

	invalidSignAlgo := runtime.SignatureAlgorithm(254)
	publicKey := &runtime.PublicKey{
		PublicKey: nil,
		SignAlgo:  invalidSignAlgo,
	}

	_, err := NewAccountPublicKey(publicKey, sema.HashAlgorithmSHA2_384, 0, 0)

	err = errors2.Unwrap(err)
	require.IsType(t, &errors.ValueError{}, err)

	require.Contains(t, err.Error(), fmt.Sprintf("%d", invalidSignAlgo))

	errorString := err.Error()
	assert.True(t, utf8.ValidString(errorString))

	// check if they can encoded and decoded using CBOR
	marshalledBytes, err := cbor.Marshal(errorString)
	require.NoError(t, err)

	var unmarshalledString string

	err = cbor.Unmarshal(marshalledBytes, &unmarshalledString)
	require.NoError(t, err)

	require.Equal(t, errorString, unmarshalledString)
}

func TestNewAccountKey_error_handling_produces_valid_utf8_and_hash_algo(t *testing.T) {

	publicKey := &runtime.PublicKey{
		PublicKey: nil,
		SignAlgo:  runtime.SignatureAlgorithmECDSA_P256,
	}

	invalidHashAlgo := sema.HashAlgorithm(112)

	_, err := NewAccountPublicKey(publicKey, invalidHashAlgo, 0, 0)

	err = errors2.Unwrap(err)
	require.IsType(t, &errors.ValueError{}, err)

	require.Contains(t, err.Error(), fmt.Sprintf("%d", invalidHashAlgo))

	errorString := err.Error()
	assert.True(t, utf8.ValidString(errorString))

	// check if they can encoded and decoded using CBOR
	marshalledBytes, err := cbor.Marshal(errorString)
	require.NoError(t, err)

	var unmarshalledString string

	err = cbor.Unmarshal(marshalledBytes, &unmarshalledString)
	require.NoError(t, err)

	require.Equal(t, errorString, unmarshalledString)
}

func TestNewAccountKey_error_handling_produces_valid_utf8(t *testing.T) {

	publicKey := &runtime.PublicKey{
		PublicKey: []byte{0xc3, 0x28}, //some invalid UTF8
		SignAlgo:  runtime.SignatureAlgorithmECDSA_P256,
	}

	_, err := NewAccountPublicKey(publicKey, runtime.HashAlgorithmSHA2_256, 0, 0)

	err = errors2.Unwrap(err)
	require.IsType(t, &errors.ValueError{}, err)

	errorString := err.Error()
	assert.True(t, utf8.ValidString(errorString))

	// check if they can encoded and decoded using CBOR
	marshalledBytes, err := cbor.Marshal(errorString)
	require.NoError(t, err)

	var unmarshalledString string

	err = cbor.Unmarshal(marshalledBytes, &unmarshalledString)
	require.NoError(t, err)

	require.Equal(t, errorString, unmarshalledString)
}

type FakePublicKey struct {
	data []byte
}

func (f FakePublicKey) Encode() []byte {
	return f.data
}

func (f FakePublicKey) Algorithm() crypto.SigningAlgorithm { return crypto.ECDSASecp256k1 }
func (f FakePublicKey) Size() int                          { return 0 }
func (f FakePublicKey) String() string                     { return "" }
func (f FakePublicKey) Verify(_ crypto.Signature, _ []byte, _ hash.Hasher) (bool, error) {
	return false, nil
}
func (f FakePublicKey) EncodeCompressed() []byte         { return nil }
func (f FakePublicKey) Equals(key crypto.PublicKey) bool { return false }

type FakeAccounts struct{}

func (f FakeAccounts) Exists(address flow.Address) (bool, error)                     { return true, nil }
func (f FakeAccounts) Get(address flow.Address) (*flow.Account, error)               { return &flow.Account{}, nil }
func (f FakeAccounts) GetPublicKeyCount(_ flow.Address) (uint64, error)              { return 0, nil }
func (f FakeAccounts) AppendPublicKey(_ flow.Address, _ flow.AccountPublicKey) error { return nil }
func (f FakeAccounts) GetPublicKey(_ flow.Address, _ uint64) (flow.AccountPublicKey, error) {
	return flow.AccountPublicKey{}, nil
}
func (f FakeAccounts) SetPublicKey(_ flow.Address, _ uint64, _ flow.AccountPublicKey) ([]byte, error) {
	return nil, nil
}
func (f FakeAccounts) GetContractNames(_ flow.Address) ([]string, error)             { return nil, nil }
func (f FakeAccounts) GetContract(_ string, _ flow.Address) ([]byte, error)          { return nil, nil }
func (f FakeAccounts) SetContract(_ string, _ flow.Address, _ []byte) error          { return nil }
func (f FakeAccounts) DeleteContract(_ string, _ flow.Address) error                 { return nil }
func (f FakeAccounts) Create(_ []flow.AccountPublicKey, _ flow.Address) error        { return nil }
func (f FakeAccounts) GetValue(_ flow.Address, _ string) (flow.RegisterValue, error) { return nil, nil }
func (f FakeAccounts) CheckAccountNotFrozen(_ flow.Address) error                    { return nil }
func (f FakeAccounts) GetStorageUsed(_ flow.Address) (uint64, error)                 { return 0, nil }
func (f FakeAccounts) SetValue(_ flow.Address, _ string, _ []byte) error             { return nil }
func (f FakeAccounts) AllocateStorageIndex(_ flow.Address) (atree.StorageIndex, error) {
	return atree.StorageIndex{}, nil
}
func (f FakeAccounts) SetAccountFrozen(_ flow.Address, _ bool) error { return nil }
