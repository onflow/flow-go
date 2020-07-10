package common_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/ledger"
	"github.com/dapperlabs/flow-go/ledger/common"
)

// Test_KeyPartEncodingDecoding tests encoding decoding functionality of a ledger key part
func Test_KeyPartEncodingDecoding(t *testing.T) {
	kpt := uint16(1)
	kpv := []byte("key part")
	kp := ledger.NewKeyPart(kpt, kpv)

	encoded := common.EncodeKeyPart(kp)
	newkp, err := common.DecodeKeyPart(encoded)
	require.NoError(t, err)
	require.Equal(t, newkp.Type, kpt)
	require.Equal(t, newkp.Value, kpv)

	// wrong type decoding
	_, err = common.DecodeKey(encoded)
	require.Error(t, err)

	// test wrong version decoding
	encoded[0] = byte(uint8(1))
	_, err = common.DecodeKeyPart(encoded)
	require.Error(t, err)
}

// Test_KeyEncodingDecoding tests encoding decoding functionality of a ledger key
func Test_KeyEncodingDecoding(t *testing.T) {
	kp1t := uint16(1)
	kp1v := []byte("key part 1")
	kp1 := ledger.NewKeyPart(kp1t, kp1v)

	kp2t := uint16(22)
	kp2v := []byte("key part 2")
	kp2 := ledger.NewKeyPart(kp2t, kp2v)

	k := ledger.NewKey([]ledger.KeyPart{*kp1, *kp2})

	encoded := common.EncodeKey(k)
	newk, err := common.DecodeKey(encoded)
	require.NoError(t, err)
	require.Equal(t, newk.KeyParts[0].Type, kp1t)
	require.Equal(t, newk.KeyParts[0].Value, kp1v)

	require.Equal(t, newk.KeyParts[1].Type, kp2t)
	require.Equal(t, newk.KeyParts[1].Value, kp2v)
}

// Test_ValueEncodingDecoding tests encoding decoding functionality of a ledger value
func Test_ValueEncodingDecoding(t *testing.T) {
	v := ledger.Value("value")

	encoded := common.EncodeValue(v)
	newV, err := common.DecodeValue(encoded)
	require.NoError(t, err)
	require.Equal(t, v, newV)

}

// Test_PayloadEncodingDecoding tests encoding decoding functionality of a payload
func Test_PayloadEncodingDecoding(t *testing.T) {
	kp1t := uint16(1)
	kp1v := []byte("key part 1")
	kp1 := ledger.NewKeyPart(kp1t, kp1v)

	kp2t := uint16(22)
	kp2v := []byte("key part 2")
	kp2 := ledger.NewKeyPart(kp2t, kp2v)

	k := ledger.NewKey([]ledger.KeyPart{*kp1, *kp2})
	v := ledger.Value([]byte{'A'})
	p := ledger.NewPayload(*k, v)

	encoded := common.EncodePayload(p)
	newp, err := common.DecodePayload(encoded)
	require.NoError(t, err)
	require.Equal(t, newp.Key.KeyParts[0].Type, kp1t)
	require.Equal(t, newp.Key.KeyParts[0].Value, kp1v)
	require.Equal(t, newp.Key.KeyParts[1].Type, kp2t)
	require.Equal(t, newp.Key.KeyParts[1].Value, kp2v)
	require.Equal(t, newp.Value, v)
}

// func TestBatchProofEncoderDecoder(t *testing.T) {
// 	pathByteSize := 1 // key size of 8 bits
// 	dir, err := ioutil.TempDir("", "test-mtrie-")
// 	require.NoError(t, err)
// 	defer os.RemoveAll(dir)

// 	metricsCollector := &metrics.NoopCollector{}
// 	fStore, err := mtrie.NewMForest(pathByteSize, dir, 5, metricsCollector, nil)
// 	require.NoError(t, err)

// 	p1 := ledger.Path([]byte{'p'})
// 	v1 := ledger.Payload{Key: ledger.Key{KeyParts: []ledger.KeyPart{ledger.KeyPart{Type: 0,
// 		Value: []byte{'k'}}}},
// 		Value: ledger.Value([]byte{'v'})}

// 	testTrie, err := fStore.Update(fStore.GetEmptyRootHash(), []ledger.Path{p1}, []ledger.Payload{v1})
// 	require.NoError(t, err)
// 	batchProof, err := fStore.Proofs(testTrie.RootHash(), []ledger.Path{p1})
// 	require.NoError(t, err)

// 	encProf, _ := batchProof.Encode()
// 	p, err := ledger.DecodeBatchProof(encProf)
// 	require.NoError(t, err)
// 	require.Equal(t, p, batchProof, "Proof encoder and/or decoder has an issue")
// }
