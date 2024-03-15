package environment_test

import (
	"fmt"
	"testing"
	"unicode/utf8"

	"github.com/fxamacker/cbor/v2"
	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
)

func TestAddEncodedAccountKey_error_handling_produces_valid_utf8(t *testing.T) {

	akh := environment.NewAccountKeyUpdater(
		tracing.NewTracerSpan(),
		nil,
		FakeAccounts{},
		nil,
		nil)

	address := flow.BytesToAddress([]byte{1, 2, 3, 4})

	// emulate encoded public key (which comes as a user input)
	// containing bytes which are invalid UTF8

	invalidEncodedKey := make([]byte, 64)
	invalidUTF8 := []byte{0xc3, 0x28}
	copy(invalidUTF8, invalidEncodedKey)
	accountPublicKey := FakePublicKey{data: invalidEncodedKey}.toAccountPublicKey()

	encodedPublicKey, err := flow.EncodeRuntimeAccountPublicKey(accountPublicKey)
	require.NoError(t, err)

	err = akh.InternalAddEncodedAccountKey(address, encodedPublicKey)
	require.Error(t, err)

	require.True(t, errors.IsValueError(err))

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

	_, err := environment.NewAccountPublicKey(
		publicKey,
		sema.HashAlgorithmSHA2_384,
		0,
		0)

	require.True(t, errors.IsValueError(err))

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

	_, err := environment.NewAccountPublicKey(publicKey, invalidHashAlgo, 0, 0)

	require.True(t, errors.IsValueError(err))

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
		PublicKey: []byte{0xc3, 0x28}, // some invalid UTF8
		SignAlgo:  runtime.SignatureAlgorithmECDSA_P256,
	}

	_, err := environment.NewAccountPublicKey(
		publicKey,
		runtime.HashAlgorithmSHA2_256,
		0,
		0)

	require.True(t, errors.IsValueError(err))

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

var _ crypto.PublicKey = &FakePublicKey{}

func (f FakePublicKey) toAccountPublicKey() flow.AccountPublicKey {
	return flow.AccountPublicKey{
		Index:     1,
		PublicKey: f,
		SignAlgo:  crypto.ECDSASecp256k1,
		HashAlgo:  hash.SHA3_256,
		SeqNumber: 0,
		Weight:    1000,
		Revoked:   false,
	}
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

type FakeAccounts struct {
	keyCount uint64
}

var _ environment.Accounts = &FakeAccounts{}

func (f FakeAccounts) Exists(address flow.Address) (bool, error)       { return true, nil }
func (f FakeAccounts) Get(address flow.Address) (*flow.Account, error) { return &flow.Account{}, nil }
func (f FakeAccounts) GetPublicKeyCount(_ flow.Address) (uint64, error) {
	return f.keyCount, nil
}
func (f FakeAccounts) AppendPublicKey(_ flow.Address, _ flow.AccountPublicKey) error { return nil }
func (f FakeAccounts) GetPublicKey(address flow.Address, keyIndex uint64) (flow.AccountPublicKey, error) {
	if keyIndex >= f.keyCount {
		return flow.AccountPublicKey{}, errors.NewAccountPublicKeyNotFoundError(address, keyIndex)
	}
	return FakePublicKey{}.toAccountPublicKey(), nil
}

func (f FakeAccounts) SetPublicKey(_ flow.Address, _ uint64, _ flow.AccountPublicKey) ([]byte, error) {
	return nil, nil
}
func (f FakeAccounts) GetContractNames(_ flow.Address) ([]string, error)      { return nil, nil }
func (f FakeAccounts) GetContract(_ string, _ flow.Address) ([]byte, error)   { return nil, nil }
func (f FakeAccounts) ContractExists(_ string, _ flow.Address) (bool, error)  { return false, nil }
func (f FakeAccounts) SetContract(_ string, _ flow.Address, _ []byte) error   { return nil }
func (f FakeAccounts) DeleteContract(_ string, _ flow.Address) error          { return nil }
func (f FakeAccounts) Create(_ []flow.AccountPublicKey, _ flow.Address) error { return nil }
func (f FakeAccounts) GetValue(_ flow.RegisterID) (flow.RegisterValue, error) { return nil, nil }
func (f FakeAccounts) GetStorageUsed(_ flow.Address) (uint64, error)          { return 0, nil }
func (f FakeAccounts) SetValue(_ flow.RegisterID, _ []byte) error             { return nil }
func (f FakeAccounts) AllocateSlabIndex(_ flow.Address) (atree.SlabIndex, error) {
	return atree.SlabIndex{}, nil
}
func (f FakeAccounts) GenerateAccountLocalID(address flow.Address) (uint64, error) {
	return 0, nil
}
