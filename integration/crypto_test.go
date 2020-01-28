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

func Test_crypto(t *testing.T) {

	privateKeyHex := "7B22507269766174654B6579223A224D48634341514545494C4B53627A43326D6E32657A6D67522B44657A31764E464133384672653751472B42536E322B69415831596F416F4743437147534D3439417745486F555144516741456533314D767A697078794261436353554D4A6944316C50783442744C4B4F5955394F5A73532F6F48646C444B312B355347664353697A4656684B7A6B587051575064622F2F37314A6F66686648622B374E57687871773D3D222C225369676E416C676F223A322C2248617368416C676F223A317D"

	nonce := 2137

	tx := sdk.Transaction{
		Script:             []byte("fun main() {}"),
		ReferenceBlockHash: []byte{1, 2, 3, 4},
		Nonce:              uint64(nonce),
		ComputeLimit:       10,
		PayerAccount:       sdk.RootAddress,
	}

	txBytes := tx.Encode()

	fmt.Printf("tx bytes: %X\n", txBytes)

	hasher, err := crypto.NewHasher(crypto.SHA2_256)
	require.NoError(t, err)

	hash := hasher.ComputeHash(txBytes)

	fmt.Printf("hash: %X\n", hash)

	seed := make([]byte, 40)
	_, _ = rand.Read(seed)

	key, err := keys.DecodePrivateKeyHex(privateKeyHex)

	//key, err := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA2_256, seed)
	require.NoError(t, err)

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

	signedHashFromJava, err := hex.DecodeString("3045022045427071A64F4DA711752F5BB63EC1EA5C797585A55970F709038C4B9158553F022100D4C351A6B9E4E071F86D3676B43507753EF250E19A24D3B4C39DB3EDB5752848")
	require.NoError(t, err)

	r  := &big.Int{}
	r.SetString("31326974518100032009599401620797971477391692715144703299492505797212195280191", 0)

	s := &big.Int{}
	s.SetString("96235422613666424450062654308260832216922088815519996699364710184878374987848", 0)

	publicKey := key.PrivateKey.PublicKey().Get()
	vv := ecdsa.Verify(publicKey, signedHashFromJava, r, s)

	fmt.Printf("data is %t\n", vv)


	v, err := key.PrivateKey.PublicKey().VerifyData(signedHashFromJava, hash)
	require.NoError(t, err)

	fmt.Printf("data is %t\n", v)


}


