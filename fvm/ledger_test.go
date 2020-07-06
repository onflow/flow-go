package fvm_test

import (
	"crypto/sha256"
	"strings"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution/testutil"
	"github.com/dapperlabs/flow-go/fvm"
	"github.com/dapperlabs/flow-go/model/flow"
)

func fullKey(owner, controller, key string) string {
	// https://en.wikipedia.org/wiki/C0_and_C1_control_codes#Field_separators
	return strings.Join([]string{owner, controller, key}, "\x1F")
}

func fullKeyHash(owner, controller, key string) flow.RegisterID {
	h := sha256.New()
	_, _ = h.Write([]byte(fullKey(owner, controller, key)))
	return h.Sum(nil)
}

func Test_AccountWithNoKeys(t *testing.T) {
	chain := flow.Mainnet.Chain()

	ledger := testutil.RootBootstrappedLedger(chain)

	rt := runtime.NewInterpreterRuntime()
	vm := fvm.New(rt, chain)

	txBody := flow.NewTransactionBody().
		SetScript(createAccountScript).
		AddAuthorizer(chain.ServiceAddress())

	ctx := fvm.NewContext(
		fvm.WithSignatureVerification(false),
		fvm.WithFeePayments(false),
		fvm.WithRestrictedAccountCreation(false),
	)

	result, err := vm.Invoke(ctx, fvm.Transaction(txBody), ledger)
	require.NoError(t, err)
	require.NoError(t, result.Error)

	address := flow.BytesToAddress(result.Events[0].Fields[0].(cadence.Address).Bytes())

	require.NotPanics(t, func() {
		_, _ = vm.GetAccount(ctx, address, ledger)
	})
}

// Some old account could be created without key count register
// we recreate it in a test
func Test_AccountWithNoKeysCounter(t *testing.T) {
	chain := flow.Mainnet.Chain()

	ledger := testutil.RootBootstrappedLedger(chain)

	rt := runtime.NewInterpreterRuntime()
	vm := fvm.New(rt, chain)

	txBody := flow.NewTransactionBody().
		SetScript(createAccountScript).
		AddAuthorizer(chain.ServiceAddress())

	ctx := fvm.NewContext(
		fvm.WithSignatureVerification(false),
		fvm.WithFeePayments(false),
		fvm.WithRestrictedAccountCreation(false),
	)

	result, err := vm.Invoke(ctx, fvm.Transaction(txBody), ledger)
	require.NoError(t, err)
	require.NoError(t, result.Error)

	address := flow.BytesToAddress(result.Events[0].Fields[0].(cadence.Address).Bytes())

	countRegister := fullKeyHash(
		string(address.Bytes()),
		string(address.Bytes()),
		"public_key_count",
	)

	ledger.Delete(countRegister)

	require.NotPanics(t, func() {
		_, _ = vm.GetAccount(ctx, address, ledger)
	})
}

func TestLedgerDAL_UUID(t *testing.T) {

	ledger := make(MapLedger)

	dal := NewLedgerDAL(ledger, flow.Testnet.Chain())

	uuid, err := dal.GetUUID() // start from zero
	require.NoError(t, err)

	require.Equal(t, uint64(0), uuid)
	dal.SetUUID(5)

	//create new ledgerDAL
	dal = NewLedgerDAL(ledger, flow.Testnet.Chain())
	uuid, err = dal.GetUUID() // should read saved value
	require.NoError(t, err)

	require.Equal(t, uint64(5), uuid)
}
