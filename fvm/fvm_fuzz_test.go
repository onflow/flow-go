package fvm_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func FuzzTransactionComputationLimit(f *testing.F) {
	// setup initial state
	vmt, tctx := bootstrapFuzzStateAndTxContext(f)

	f.Add(uint64(0), uint64(0), uint64(0), uint(0))
	f.Add(uint64(5), uint64(0), uint64(0), uint(0))
	f.Fuzz(func(t *testing.T, computationLimit uint64, memoryLimit uint64, interactionLimit uint64, transactionType uint) {
		computationLimit %= flow.DefaultMaxTransactionGasLimit
		transactionType %= uint(len(fuzzTransactionTypes))

		tt := fuzzTransactionTypes[transactionType]

		vmt.run(func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
			// create the transaction
			txBody := tt.createTxBody(t, tctx)
			// set the computation limit
			txBody.SetGasLimit(computationLimit)

			// sign the transaction
			err := testutil.SignEnvelope(
				txBody,
				tctx.address,
				tctx.privateKey,
			)
			require.NoError(t, err)

			// set the memory limit
			ctx.MemoryLimit = memoryLimit
			// set the interaction limit
			ctx.MaxStateInteractionSize = interactionLimit

			var output fvm.ProcedureOutput

			// run the transaction
			require.NotPanics(t, func() {
				_, output, err = vm.Run(
					ctx,
					fvm.Transaction(txBody, 0),
					snapshotTree)
			}, "Transaction should never result in a panic.")
			require.NoError(t, err, "Transaction should never result in an error.")

			// check if results are expected
			tt.require(t, tctx, fuzzResults{
				output: output,
			})
		})(t)
	})
}

type fuzzResults struct {
	output fvm.ProcedureOutput
}

type transactionTypeContext struct {
	address      flow.Address
	addressFunds uint64
	privateKey   flow.AccountPrivateKey
	chain        flow.Chain
}

type transactionType struct {
	createTxBody func(t *testing.T, tctx transactionTypeContext) *flow.TransactionBody
	require      func(t *testing.T, tctx transactionTypeContext, results fuzzResults)
}

var fuzzTransactionTypes = []transactionType{
	{
		// Token transfer of 0 tokens.
		// should succeed if no limits are hit.
		// fees should be deducted no matter what.
		createTxBody: func(t *testing.T, tctx transactionTypeContext) *flow.TransactionBody {
			txBody := transferTokensTx(tctx.chain).
				AddAuthorizer(tctx.address).
				AddArgument(jsoncdc.MustEncode(cadence.UFix64(0))). // 0 value transferred
				AddArgument(jsoncdc.MustEncode(cadence.NewAddress(tctx.chain.ServiceAddress())))

			txBody.SetProposalKey(tctx.address, 0, 0)
			txBody.SetPayer(tctx.address)
			return txBody
		},
		require: func(t *testing.T, tctx transactionTypeContext, results fuzzResults) {
			// if there is an error, it should be computation exceeded
			if results.output.Err != nil {
				require.Len(t, results.output.Events, 5)
				unittest.EnsureEventsIndexSeq(t, results.output.Events, tctx.chain.ChainID())
				codes := []errors.ErrorCode{
					errors.ErrCodeComputationLimitExceededError,
					errors.ErrCodeCadenceRunTimeError,
					errors.ErrCodeLedgerInteractionLimitExceededError,
				}
				require.Contains(t, codes, results.output.Err.Code(), results.output.Err.Error())
			}

			// fees should be deducted no matter the input
			fees, deducted := getDeductedFees(t, tctx, results)
			require.True(t, deducted, "Fees should be deducted.")
			require.GreaterOrEqual(t, fees.ToGoValue().(uint64), fuzzTestsInclusionFees)
			unittest.EnsureEventsIndexSeq(t, results.output.Events, tctx.chain.ChainID())
		},
	},
	{
		// Token transfer of too many tokens.
		// Should never succeed.
		// fees should be deducted no matter what.
		createTxBody: func(t *testing.T, tctx transactionTypeContext) *flow.TransactionBody {
			txBody := transferTokensTx(tctx.chain).
				AddAuthorizer(tctx.address).
				AddArgument(jsoncdc.MustEncode(cadence.UFix64(2 * tctx.addressFunds))). // too much value transferred
				AddArgument(jsoncdc.MustEncode(cadence.NewAddress(tctx.chain.ServiceAddress())))

			txBody.SetProposalKey(tctx.address, 0, 0)
			txBody.SetPayer(tctx.address)
			return txBody
		},
		require: func(t *testing.T, tctx transactionTypeContext, results fuzzResults) {
			require.Error(t, results.output.Err)
			require.Len(t, results.output.Events, 3)
			unittest.EnsureEventsIndexSeq(t, results.output.Events, tctx.chain.ChainID())
			codes := []errors.ErrorCode{
				errors.ErrCodeComputationLimitExceededError,
				errors.ErrCodeCadenceRunTimeError, // because of the failed transfer
				errors.ErrCodeLedgerInteractionLimitExceededError,
			}
			require.Contains(t, codes, results.output.Err.Code(), results.output.Err.Error())

			// fees should be deducted no matter the input
			fees, deducted := getDeductedFees(t, tctx, results)
			require.True(t, deducted, "Fees should be deducted.")
			require.GreaterOrEqual(t, fees.ToGoValue().(uint64), fuzzTestsInclusionFees)
			unittest.EnsureEventsIndexSeq(t, results.output.Events, tctx.chain.ChainID())
		},
	},
	{
		// Transaction that calls panic.
		// Should never succeed.
		// fees should be deducted no matter what.
		createTxBody: func(t *testing.T, tctx transactionTypeContext) *flow.TransactionBody {
			// empty transaction
			txBody := flow.NewTransactionBody().SetScript([]byte("transaction(){prepare(){};execute{panic(\"some panic\")}}"))
			txBody.SetProposalKey(tctx.address, 0, 0)
			txBody.SetPayer(tctx.address)
			return txBody
		},
		require: func(t *testing.T, tctx transactionTypeContext, results fuzzResults) {
			require.Error(t, results.output.Err)
			require.Len(t, results.output.Events, 3)
			unittest.EnsureEventsIndexSeq(t, results.output.Events, tctx.chain.ChainID())
			codes := []errors.ErrorCode{
				errors.ErrCodeComputationLimitExceededError,
				errors.ErrCodeCadenceRunTimeError, // because of the panic
				errors.ErrCodeLedgerInteractionLimitExceededError,
			}
			require.Contains(t, codes, results.output.Err.Code(), results.output.Err.Error())

			// fees should be deducted no matter the input
			fees, deducted := getDeductedFees(t, tctx, results)
			require.True(t, deducted, "Fees should be deducted.")
			require.GreaterOrEqual(t, fees.ToGoValue().(uint64), fuzzTestsInclusionFees)
			unittest.EnsureEventsIndexSeq(t, results.output.Events, tctx.chain.ChainID())
		},
	},
	{
		createTxBody: func(t *testing.T, tctx transactionTypeContext) *flow.TransactionBody {
			// create account
			txBody := flow.NewTransactionBody().SetScript(createAccountScript).
				AddAuthorizer(tctx.address)
			txBody.SetProposalKey(tctx.address, 0, 0)
			txBody.SetPayer(tctx.address)
			return txBody
		},
		require: func(t *testing.T, tctx transactionTypeContext, results fuzzResults) {
			// if there is an error, it should be computation exceeded
			if results.output.Err != nil {
				require.Len(t, results.output.Events, 3)
				unittest.EnsureEventsIndexSeq(t, results.output.Events, tctx.chain.ChainID())
				codes := []errors.ErrorCode{
					errors.ErrCodeComputationLimitExceededError,
					errors.ErrCodeCadenceRunTimeError,
					errors.ErrCodeLedgerInteractionLimitExceededError,
				}
				require.Contains(t, codes, results.output.Err.Code(), results.output.Err.Error())
			}

			// fees should be deducted no matter the input
			fees, deducted := getDeductedFees(t, tctx, results)
			require.True(t, deducted, "Fees should be deducted.")
			require.GreaterOrEqual(t, fees.ToGoValue().(uint64), fuzzTestsInclusionFees)
			unittest.EnsureEventsIndexSeq(t, results.output.Events, tctx.chain.ChainID())
		},
	},
}

const fuzzTestsInclusionFees = uint64(1_000)

// checks fee deduction happened and returns the amount of funds deducted
func getDeductedFees(tb testing.TB, tctx transactionTypeContext, results fuzzResults) (fees cadence.UFix64, deducted bool) {
	tb.Helper()

	var ok bool
	var feesDeductedEvent cadence.Event
	for _, e := range results.output.Events {
		if string(e.Type) == fmt.Sprintf("A.%s.FlowFees.FeesDeducted", environment.FlowFeesAddress(tctx.chain)) {
			data, err := ccf.Decode(nil, e.Payload)
			require.NoError(tb, err)
			feesDeductedEvent, ok = data.(cadence.Event)
			require.True(tb, ok, "Event payload should be of type cadence event.")
		}
	}
	if feesDeductedEvent.Type() == nil {
		return 0, false
	}

	for i, f := range feesDeductedEvent.Type().(*cadence.EventType).Fields {
		if f.Identifier == "amount" {
			fees, ok = feesDeductedEvent.Fields[i].(cadence.UFix64)
			require.True(tb, ok, "FeesDeducted event amount field should be of type cadence.UFix64.")
			break
		}
	}

	return fees, true
}

func bootstrapFuzzStateAndTxContext(tb testing.TB) (bootstrappedVmTest, transactionTypeContext) {
	tb.Helper()

	addressFunds := uint64(1_000_000_000)
	var privateKey flow.AccountPrivateKey
	var address flow.Address
	bootstrappedVMTest, err := newVMTest().withBootstrapProcedureOptions(
		fvm.WithTransactionFee(fvm.DefaultTransactionFees),
		fvm.WithExecutionMemoryLimit(math.MaxUint32),
		fvm.WithExecutionEffortWeights(mainnetExecutionEffortWeights),
		fvm.WithExecutionMemoryWeights(meter.DefaultMemoryWeights),
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
	).withContextOptions(
		fvm.WithTransactionFeesEnabled(true),
		fvm.WithAccountStorageLimit(true),
	).bootstrapWith(func(vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) (snapshot.SnapshotTree, error) {
		// ==== Create an account ====
		var txBody *flow.TransactionBody
		privateKey, txBody = testutil.CreateAccountCreationTransaction(tb, chain)

		err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
		if err != nil {
			return snapshotTree, err
		}

		executionSnapshot, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(tb, err)
		require.NoError(tb, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

		accountCreatedEvents := filterAccountCreatedEvents(output.Events)

		// read the address of the account created (e.g. "0x01" and convert it to flow.address)
		data, err := ccf.Decode(nil, accountCreatedEvents[0].Payload)
		require.NoError(tb, err)

		address = flow.ConvertAddress(
			data.(cadence.Event).Fields[0].(cadence.Address))

		// ==== Transfer tokens to new account ====
		txBody = transferTokensTx(chain).
			AddAuthorizer(chain.ServiceAddress()).
			AddArgument(jsoncdc.MustEncode(cadence.UFix64(1_000_000_000))). // 10 FLOW
			AddArgument(jsoncdc.MustEncode(cadence.NewAddress(address)))

		txBody.SetProposalKey(chain.ServiceAddress(), 0, 1)
		txBody.SetPayer(chain.ServiceAddress())

		err = testutil.SignEnvelope(
			txBody,
			chain.ServiceAddress(),
			unittest.ServiceAccountPrivateKey,
		)
		require.NoError(tb, err)

		executionSnapshot, output, err = vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		if err != nil {
			return snapshotTree, err
		}

		return snapshotTree.Append(executionSnapshot), output.Err
	})
	require.NoError(tb, err)

	return bootstrappedVMTest,
		transactionTypeContext{
			address:      address,
			addressFunds: addressFunds,
			privateKey:   privateKey,
			chain:        bootstrappedVMTest.chain,
		}
}
