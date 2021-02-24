package flow

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
)

// This tests are not really test, just a handy methods to generate outputs
// to be used in other tests - for example in JVM SDK
func Test_TransactionCanonicalForm(t *testing.T) {

	tb := NewTransactionBody()
	tb.Arguments = [][]byte{{2, 2, 3}, {3, 3, 3}}
	tb.Authorizers = []Address{
		{0, 0, 0, 9, 9, 9, 9, 9},
		{0, 0, 0, 8, 9, 9, 9, 9},
	}
	tb.ProposalKey = ProposalKey{
		Address:        Address{0, 0, 4, 5, 4, 5, 4, 5},
		KeyIndex:       11,
		SequenceNumber: 7,
	}
	tb.Payer = Address{0, 0, 0, 6, 5, 4, 3, 2}
	tb.GasLimit = 44
	tb.Script = []byte("import 0xsomething \n {}")
	tb.ReferenceBlockID = Identifier{
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 3, 3, 3, 6, 6, 6,
	}

	payloadMessage := tb.PayloadMessage()

	fmt.Printf("payload message hex: %s\n", hex.EncodeToString(payloadMessage))

	address := HexToAddress("f8d6e0586b0a20c7")

	tb.PayloadSignatures = append(tb.PayloadSignatures, TransactionSignature{
		Address:     address,
		SignerIndex: 0,
		KeyIndex:    0,
		Signature:   []byte{4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4},
	})

	tb.PayloadSignatures = append(tb.PayloadSignatures, TransactionSignature{
		Address:     address,
		SignerIndex: 4,
		KeyIndex:    5,
		Signature:   []byte{3, 3, 3},
	})

	envelopeMessage := tb.EnvelopeMessage()

	fmt.Printf("envelope message hex: %s\n", hex.EncodeToString(envelopeMessage))
}

func Test_TransactionVerify(t *testing.T) {
	privKeyBytes, err := hex.DecodeString("749024acd97d0b72448f1baf600314cfc28d97a4e1816e0578f4ae3befcf4e26")
	require.NoError(t, err)

	privKey, err := crypto.DecodePrivateKey(crypto.ECDSAP256, privKeyBytes)
	require.NoError(t, err)

	// FILL THOSE
	payloadSignature, err := hex.DecodeString("6bf7923601c94b1fc2b47b8342b13ed2a97dd657507b41cd49b82e246102982a311c3c660de8cbd40f5aba3dcb56e87d5e24743e49f471c4c691baab26eb6ec0")
	require.NoError(t, err)

	envelopeSignature, err := hex.DecodeString("74fa94ef620d7045fe4960d964a5ee1a6113828493328c7e548c96e344e9a3d0697d6d23ca11eee557ef5bb41a2ba639c172f13c6b31cbcbe5a1a08ef5fd24fa")
	require.NoError(t, err)
	// END FILL

	tb := NewTransactionBody()
	tb.Arguments = [][]byte{{2, 2, 3}, {3, 3, 3}}
	tb.Authorizers = []Address{
		{0, 0, 0, 9, 9, 9, 9, 9},
		{0, 0, 0, 8, 9, 9, 9, 9},
	}
	tb.ProposalKey = ProposalKey{
		Address:        Address{0, 0, 4, 5, 4, 5, 4, 5},
		KeyIndex:       11,
		SequenceNumber: 7,
	}
	tb.Payer = Address{0, 0, 0, 6, 5, 4, 3, 2}
	tb.GasLimit = 44
	tb.Script = []byte("import 0xsomething \n {}")
	tb.ReferenceBlockID = Identifier{
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 3, 3, 3, 6, 6, 6,
	}

	address := HexToAddress("f8d6e0586b0a20c7")

	tb.PayloadSignatures = append(tb.PayloadSignatures, TransactionSignature{
		Address:     address,
		SignerIndex: 0,
		KeyIndex:    0,
		Signature:   []byte{4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4},
	})

	tb.PayloadSignatures = append(tb.PayloadSignatures, TransactionSignature{
		Address:     address,
		SignerIndex: 4,
		KeyIndex:    5,
		Signature:   []byte{3, 3, 3},
	})

	hasher := hash.NewSHA3_256()
	publicKey := privKey.PublicKey()

	payloadMessage := tb.PayloadMessage()
	fmt.Printf("Payload message: %s\n", hex.EncodeToString(payloadMessage))
	envelopMessage := tb.EnvelopeMessage()

	payloadVerify, err := publicKey.Verify(payloadSignature, payloadMessage, hasher)
	require.NoError(t, err)

	envelopeVerify, err := publicKey.Verify(envelopeSignature, envelopMessage, hasher)
	require.NoError(t, err)

	fmt.Printf("Payload verify %t\n", payloadVerify)
	fmt.Printf("Envelope verify %t\n", envelopeVerify)

	assert.True(t, payloadVerify)
	assert.True(t, envelopeVerify)

	// RS testing sections

	//type ecdsaSignature struct {
	//	R *big.Int
	//	S *big.Int
	//}

	//var esig ecdsaSignature
	//asn1.Unmarshal(signature, &esig)
	//fmt.Printf("R: %d , S: %d\n", esig.R, esig.S)

	//pk := publicKey.(*crypto.PubKeyECDSA)
	//
	//r := big.NewInt(0)
	//s := big.NewInt(0)
	//
	//r.SetString("12834344295352364623782965749753903959194793951360230632112582769878055570205", 10)
	//s.SetString("80396491735531384066734574087522051369988225701710966841735950077624757997950", 10)
	//
	//rBytes := r.Bytes()
	//sBytes := s.Bytes()
	//
	//fmt.Printf("R bytes (%d): %s\n", len(rBytes), hex.EncodeToString(rBytes))
	//fmt.Printf("S bytes (%d): %s\n", len(sBytes), hex.EncodeToString(sBytes))
	//
	//Nlen := 32
	//ss := make([]byte, 2*Nlen)
	//// pad the signature with zeroes
	//copy(ss[Nlen-len(rBytes):], rBytes)
	//copy(ss[2*Nlen-len(sBytes):], sBytes)
	//
	//fmt.Printf("Magic RS concat = %s\n", hex.EncodeToString(ss))
	//
	//v := ecdsa.Verify(pk.PK(), hasher.ComputeHash(payloadMessage), r, s)
	//
	//fmt.Printf("VerifyRS = %t\n", v)

	// end of RS testing section
}
