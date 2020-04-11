package tests

import (
	"bytes"
	"context"
	"crypto/rand"

	encoding "github.com/dapperlabs/cadence/encoding/xdr"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/integration/dsl"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/model/flow"
)

func readCounter(ctx context.Context, client *testnet.Client) (int, error) {

	script := dsl.Main{
		ReturnType: "Int",
		Code:       "return getAccount(0x01).published[&Testing.Counter]?.count ?? -3",
	}

	res, err := client.ExecuteScript(ctx, script)
	if err != nil {
		return 0, err
	}

	decoder := encoding.NewDecoder(bytes.NewReader(res))
	i, err := decoder.DecodeInt()
	if err != nil {
		return 0, err
	}

	return int(i.Value.Int64()), nil
}

func createCounter(ctx context.Context, client *testnet.Client) error {
	return client.SendTransaction(ctx, dsl.Transaction{
		Import: dsl.Import{Address: flow.RootAddress},
		Content: dsl.Prepare{
			Content: dsl.Code(`
				if signer.storage[Testing.Counter] == nil {
				let existing <- signer.storage[Testing.Counter] <- Testing.createCounter()
            	    destroy existing
            	    signer.published[&Testing.Counter] = &signer.storage[Testing.Counter] as Testing.Counter
            	}
            	signer.published[&Testing.Counter]?.add(2)`),
		}})

}

func noopTransaction() dsl.Transaction {
	return dsl.Transaction{
		Import: dsl.Import{Address: flow.RootAddress},
		Content: dsl.Prepare{
			Content: dsl.Code(`
				pub fun main() {}
			`),
		},
	}
}

func generateRandomKey() (*flow.AccountPrivateKey, error) {
	seed := make([]byte, crypto.KeyGenSeedMinLenEcdsaP256)
	n, err := rand.Read(seed)
	if err != nil || n != crypto.KeyGenSeedMinLenEcdsaP256 {
		return nil, err
	}
	key, err := crypto.GeneratePrivateKey(crypto.EcdsaP256, seed)
	if err != nil {
		return nil, err
	}

	return &flow.AccountPrivateKey{
		PrivateKey: key,
		SignAlgo:   key.Algorithm(),
		HashAlgo:   hash.SHA3_256,
	}, nil

}

func getEmulatorKey() (*flow.AccountPrivateKey, error) {
	key, err := crypto.DecodePrivateKey(crypto.EcdsaP256, []byte("f87db87930770201010420ae2cc975dcbdd0ebc56f268b1d8a95834c2955970aea27042d35ec9f298b9e5aa00a06082a8648ce3d030107a1440342000417f5a527137785d2d773fee84b4c7ee40266a1dd1f36ddd46ecf25db6df6a499459629174de83256f2a44ebd4325b9def67d523b755a8926218c4efb7904f8ce0203"))
	if err != nil {
		return nil, err
	}
	return &flow.AccountPrivateKey{
		PrivateKey: key,
		SignAlgo:   key.Algorithm(),
		HashAlgo:   hash.SHA3_256,
	}, nil
}
