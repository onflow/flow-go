package request

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
)

func TestSignature_InvalidParse(t *testing.T) {
	var signature Signature

	tests := []struct {
		in  string
		err string
	}{
		{"s", "invalid encoding"},
		{"", "missing value"},
	}

	for _, test := range tests {
		err := signature.Parse(test.in)
		assert.EqualError(t, err, test.err)
	}

}

func TestSignature_ValidParse(t *testing.T) {
	var signature Signature
	err := signature.Parse("Mzg0MTQ5ODg4ZTg4MjRmYjMyNzM4MmM2ZWQ4ZjNjZjk1ODRlNTNlMzk4NGNhMDAxZmZjMjgwNzM4NmM0MzY3NTYxNmYwMTAwMTMzNDVkNjhmNzZkMmQ5YTBkYmI1MDA0MmEzOWRlOThlYzAzNTJjYTBkZWY3YjBlNjQ0YWJjOTQ=")
	assert.NoError(t, err)
	// todo test values
}

func TestTransactionSignature_ValidParse(t *testing.T) {
	var txSignature TransactionSignature
	addr := "01cf0e2f2f715450"
	sig := "c83665f5212fad065cd27d370ef80e5fbdd20cd57411af5c76076a15dced05ac6e6d9afa88cd7337bf9c869f6785ecc1c568ca593a99dfeec14e024c0cd78289"
	sigHex, _ := hex.DecodeString(sig)
	encodedSig := util.ToBase64(sigHex)
	err := txSignature.Parse(addr, "0", encodedSig, flow.Localnet.Chain())

	assert.NoError(t, err)
	assert.Equal(t, addr, txSignature.Address.String())
	assert.Equal(t, 0, txSignature.SignerIndex)
	assert.Equal(t, uint64(0), txSignature.KeyIndex)
	assert.Equal(t, sig, fmt.Sprintf("%x", txSignature.Signature))
}

func TestTransactionSignatures_ValidParse(t *testing.T) {
	tests := []struct {
		inAddresses []string
		inSigs      []string
	}{
		{[]string{"01cf0e2f2f715450"}, []string{"c83665f5212fad065cd27d370ef80e5fbdd20cd57411af5c76076a15dced05ac6e6d9afa88cd7337bf9c869f6785ecc1c568ca593a99dfeec14e024c0cd78289"}},
		{[]string{"ee82856bf20e2aa6", "e03daebed8ca0615"}, []string{"223665f5212fad065cd27d370ef80e5fbdd20cd57411af5c76076a15dced05ac6e6d9afa88cd7337bf9c869f6785ecc1c568ca593a99dfeec14e024c0cd78289", "5553665f5212fad065cd27d370ef80e5fbdd20cd57411af5c76076a15dced05ac6e6d9afa88cd7337bf9c869f6785ecc1c568ca593a99dfeec14e024c0cd7822"}},
	}

	var txSigantures TransactionSignatures
	chain := flow.Localnet.Chain()
	for _, test := range tests {
		sigs := make([]models.TransactionSignature, len(test.inAddresses))
		for i, a := range test.inAddresses {
			sigHex, _ := hex.DecodeString(test.inSigs[i])
			encodedSig := util.ToBase64(sigHex)
			sigs[i].Signature = encodedSig
			sigs[i].KeyIndex = "0"
			sigs[i].Address = a
		}

		err := txSigantures.Parse(sigs, chain)
		assert.NoError(t, err)

		assert.Equal(t, len(txSigantures), len(sigs))
		for i, sig := range sigs {
			assert.Equal(t, sig.Address, txSigantures[i].Address.String())
			assert.Equal(t, 0, txSigantures[i].SignerIndex)
			assert.Equal(t, uint64(0), txSigantures[i].KeyIndex)
			assert.Equal(t, test.inSigs[i], fmt.Sprintf("%x", txSigantures[i].Signature))
		}
	}
}
