package testutils

import (
	"bytes"
	"crypto/ecdsa"
	"io"
	"math/big"
	"sync"
	"testing"

	gethCommon "github.com/onflow/go-ethereum/common"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	gethCrypto "github.com/onflow/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/onflow/atree"

	"github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

// address:  658bdf435d810c91414ec09147daa6db62406379
const EOATestAccount1KeyHex = "9c647b8b7c4e7c3490668fb6c11473619db80c93704c70893d3813af4090c39c"

type EOATestAccount struct {
	address gethCommon.Address
	key     *ecdsa.PrivateKey
	nonce   uint64
	signer  gethTypes.Signer
	lock    sync.Mutex
}

func (a *EOATestAccount) Address() types.Address {
	return types.Address(a.address)
}

func (a *EOATestAccount) PrepareSignAndEncodeTx(
	t testing.TB,
	to gethCommon.Address,
	data []byte,
	amount *big.Int,
	gasLimit uint64,
	gasPrice *big.Int,
) []byte {
	tx := a.PrepareAndSignTx(t, to, data, amount, gasLimit, gasPrice)
	var b bytes.Buffer
	writer := io.Writer(&b)
	err := tx.EncodeRLP(writer)
	require.NoError(t, err)
	return b.Bytes()
}

func (a *EOATestAccount) PrepareAndSignTx(
	t testing.TB,
	to gethCommon.Address,
	data []byte,
	amount *big.Int,
	gasLimit uint64,
	gasPrice *big.Int,
) *gethTypes.Transaction {
	a.lock.Lock()
	defer a.lock.Unlock()

	tx := a.signTx(
		t,
		gethTypes.NewTransaction(
			a.nonce,
			to,
			amount,
			gasLimit,
			gasPrice,
			data,
		),
	)
	a.nonce++

	return tx
}

func (a *EOATestAccount) SignTx(
	t testing.TB,
	tx *gethTypes.Transaction,
) *gethTypes.Transaction {
	a.lock.Lock()
	defer a.lock.Unlock()

	return a.signTx(t, tx)
}

func (a *EOATestAccount) signTx(
	t testing.TB,
	tx *gethTypes.Transaction,
) *gethTypes.Transaction {
	tx, err := gethTypes.SignTx(tx, a.signer, a.key)
	require.NoError(t, err)
	return tx
}

func (a *EOATestAccount) Nonce() uint64 {
	return a.nonce
}

func (a *EOATestAccount) SetNonce(nonce uint64) {
	a.lock.Lock()
	defer a.lock.Unlock()

	a.nonce = nonce
}

func GetTestEOAAccount(t testing.TB, keyHex string) *EOATestAccount {
	key, _ := gethCrypto.HexToECDSA(keyHex)
	address := gethCrypto.PubkeyToAddress(key.PublicKey)
	signer := emulator.GetDefaultSigner()
	return &EOATestAccount{
		address: address,
		key:     key,
		signer:  signer,
		lock:    sync.Mutex{},
	}
}

func RunWithEOATestAccount(t testing.TB, led atree.Ledger, flowEVMRootAddress flow.Address, f func(*EOATestAccount)) {
	account := FundAndGetEOATestAccount(t, led, flowEVMRootAddress)
	f(account)
}

func FundAndGetEOATestAccount(t testing.TB, led atree.Ledger, flowEVMRootAddress flow.Address) *EOATestAccount {
	account := GetTestEOAAccount(t, EOATestAccount1KeyHex)

	// fund account
	e := emulator.NewEmulator(led, flowEVMRootAddress)

	blk, err := e.NewBlockView(types.NewDefaultBlockContext(2))
	require.NoError(t, err)

	_, err = blk.DirectCall(
		types.NewDepositCall(
			RandomAddress(t), // any random non-empty address works here
			account.Address(),
			new(big.Int).Mul(big.NewInt(1e18), big.NewInt(1000)),
			account.nonce,
		),
	)
	require.NoError(t, err)

	blk2, err := e.NewReadOnlyBlockView(types.NewDefaultBlockContext(2))
	require.NoError(t, err)

	bal, err := blk2.BalanceOf(account.Address())
	require.NoError(t, err)
	require.Greater(t, bal.Uint64(), uint64(0))

	return account
}
