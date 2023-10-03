package testutils

import (
	"bytes"
	"crypto/ecdsa"
	"io"
	"math/big"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/onflow/atree"
	env "github.com/onflow/flow-go/fvm/flex/environment"
	"github.com/onflow/flow-go/fvm/flex/models"
	"github.com/onflow/flow-go/fvm/flex/storage"
	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/require"
)

// address:  658bdf435d810c91414ec09147daa6db62406379
const EOATestAccount1KeyHex = "9c647b8b7c4e7c3490668fb6c11473619db80c93704c70893d3813af4090c39c"

type EOATestAccount struct {
	address common.Address
	key     *ecdsa.PrivateKey
	nonce   uint64
	signer  types.Signer
	lock    sync.Mutex
}

func (a *EOATestAccount) FlexAddress() models.FlexAddress {
	return models.FlexAddress(a.address)
}

func (a *EOATestAccount) PrepareSignAndEncodeTx(
	t testing.TB,
	to common.Address,
	data []byte,
	amount *big.Int,
	gasLimit uint64,
	gasFee *big.Int,
) []byte {
	tx := a.PrepareAndSignTx(t, to, data, amount, gasLimit, gasFee)
	var b bytes.Buffer
	writer := io.Writer(&b)
	tx.EncodeRLP(writer)
	return b.Bytes()
}

func (a *EOATestAccount) PrepareAndSignTx(
	t testing.TB,
	to common.Address,
	data []byte,
	amount *big.Int,
	gasLimit uint64,
	gasFee *big.Int,
) *types.Transaction {

	a.lock.Lock()
	defer a.lock.Unlock()

	tx, err := types.SignTx(
		types.NewTransaction(
			a.nonce,
			to,
			amount,
			gasLimit,
			gasFee,
			data),
		a.signer, a.key)
	require.NoError(t, err)
	a.nonce++

	return tx
}

func GetTestEOAAccount(t testing.TB, keyHex string) *EOATestAccount {
	key, _ := crypto.HexToECDSA(keyHex)
	address := crypto.PubkeyToAddress(key.PublicKey)
	flexConf := env.NewFlexConfig()
	signer := types.MakeSigner(flexConf.ChainConfig, env.BlockNumberForEVMRules, flexConf.BlockContext.Time)

	return &EOATestAccount{
		address: address,
		key:     key,
		signer:  signer,
		lock:    sync.Mutex{},
	}
}

func RunWithEOATestAccount(t *testing.T, led atree.Ledger, flexRoot flow.Address, f func(*EOATestAccount)) {
	account := GetTestEOAAccount(t, EOATestAccount1KeyHex)

	// fund account
	db := storage.NewDatabase(led, flexRoot)
	config := env.NewFlexConfig(env.WithBlockNumber(env.BlockNumberForEVMRules))

	e, err := env.NewEnvironment(config, db)
	require.NoError(t, err)

	err = e.MintTo(new(big.Int).Mul(big.NewInt(1e18), big.NewInt(1000)), account.FlexAddress().ToCommon())
	require.NoError(t, err)
	require.False(t, e.Result.Failed)

	f(account)
}
