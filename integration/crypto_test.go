package integration

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	sdk "github.com/dapperlabs/flow-go-sdk"
	"github.com/dapperlabs/flow-go-sdk/keys"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
)

var nonce = 2137

var tx = sdk.Transaction{
	Script:             []byte("fun main() {}"),
	ReferenceBlockHash: []byte{1, 2, 3, 4},
	Nonce:              uint64(nonce),
	ComputeLimit:       10,
	PayerAccount:       sdk.RootAddress,
}

var hasher, _ = crypto.NewHasher(crypto.SHA2_256)

var hash = hasher.ComputeHash(txBytes)

var txBytes = tx.Encode()

var privateKeyHex = "7B22507269766174654B6579223A224D48634341514545494C4B53627A43326D6E32657A6D67522B44657A31764E464133384672653751472B42536E322B69415831596F416F4743437147534D3439417745486F555144516741456533314D767A697078794261436353554D4A6944316C50783442744C4B4F5955394F5A73532F6F48646C444B312B355347664353697A4656684B7A6B587051575064622F2F37314A6F66686648622B374E57687871773D3D222C225369676E416C676F223A322C2248617368416C676F223A317D"
var key, _ = keys.DecodePrivateKeyHex(privateKeyHex)

func Test_crypto(t *testing.T) {

	fmt.Printf("tx bytes: %X\n", txBytes)

	fmt.Printf("hash: %X\n", hash)

	seed := make([]byte, 40)
	_, _ = rand.Read(seed)

	//key, err := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA2_256, seed)

	pkBytes, err := key.PrivateKey.Encode()
	require.NoError(t, err)

	fmt.Printf("pk: %X\n", pkBytes)

	accountPrivateKey, err := keys.EncodePrivateKey(key)
	require.NoError(t, err)

	fmt.Printf("account pk: %X\n", accountPrivateKey)

	signedBytes, err := key.PrivateKey.SignData(hash)
	require.NoError(t, err)

	fmt.Printf("signed : %X\n", signedBytes)

	signedBytes2, err := key.PrivateKey.SignData(hash)
	require.NoError(t, err)

	fmt.Printf("signed2: %s\n", hex.EncodeToString(signedBytes2))

	signedBytes3, err := key.PrivateKey.SignData(hash)
	require.NoError(t, err)

	fmt.Printf("signed3: %X\n", signedBytes3)

	signedHashFromJava, err := hex.DecodeString("30440220587BD51E10AD3381E86B9906E7529FF0E9F1AB36C13FDE14B8E43605D185A925022021AA8BE28BC624A793B3E8275FDD9D002E2436066BA36DD7D5057A87676168FC")
	require.NoError(t, err)

	r := &big.Int{}
	r.SetString("e808d7dc7bd081742de7223afaf6658ee527911bb3ca4d1387c3e0ad453829b5", 16)

	s := &big.Int{}
	s.SetString("79862323f6bcba44fa58604a07d165dbd8616189d6ef3c2fb84b5a80ed1edf67", 16)

	//aaaHash := hasher.ComputeHash([]byte("aaa"))

	publicKey := key.PrivateKey.PublicKey().Get()
	vv := ecdsa.Verify(publicKey, hash, r, s)

	fmt.Printf("data1 is %t\n", vv)

	v, err := key.PrivateKey.PublicKey().VerifyData(signedHashFromJava, hash)
	require.NoError(t, err)

	fmt.Printf("data2 is %t\n", v)

}

func Test_crypto2(t *testing.T) {

	seed := make([]byte, 40)
	_, _ = rand.Read(seed)

	data, _ := key.PrivateKey.SignData([]byte("aaa"))

	fmt.Printf("signed: %s\n", hex.EncodeToString(data))

}
