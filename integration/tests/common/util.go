package testutil

import (
	"bytes"
	"context"

	encoding "github.com/dapperlabs/cadence/encoding/xdr"

	"github.com/dapperlabs/flow-go/integration/dsl"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/model/flow"
)

func ReadCounter(ctx context.Context, client *testnet.Client) (int, error) {

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

func CreateCounter(ctx context.Context, client *testnet.Client) error {
	return client.SendTransaction(ctx, dsl.Transaction{
		Import: dsl.Import{Address: flow.RootAddress},
		Content: dsl.Prepare{
			Content: dsl.Code(`
				if signer.storage[Testing.Counter] == nil {
				let existing <- signer.storage[Testing.Counter] <- Testing.CreateCounter()
            	    destroy existing
            	    signer.published[&Testing.Counter] = &signer.storage[Testing.Counter] as Testing.Counter
            	}
            	signer.published[&Testing.Counter]?.add(2)`),
		}})

}
