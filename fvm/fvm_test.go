package fvm_test

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"

	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/engine/execution/testutil"
	exeUtils "github.com/onflow/flow-go/engine/execution/utils"
	"github.com/onflow/flow-go/fvm"
	fvmCrypto "github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/environment"
	errors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter"
	reusableRuntime "github.com/onflow/flow-go/fvm/runtime"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// from 18.8.2022
var mainnetExecutionEffortWeights = meter.ExecutionEffortWeights{
	common.ComputationKindStatement:          1569,
	common.ComputationKindLoop:               1569,
	common.ComputationKindFunctionInvocation: 1569,
	environment.ComputationKindGetValue:      808,
	environment.ComputationKindCreateAccount: 2837670,
	environment.ComputationKindSetValue:      765,
}

type vmTest struct {
	bootstrapOptions []fvm.BootstrapProcedureOption
	contextOptions   []fvm.Option
}

func newVMTest() vmTest {
	return vmTest{}
}

func (vmt vmTest) withBootstrapProcedureOptions(opts ...fvm.BootstrapProcedureOption) vmTest {
	vmt.bootstrapOptions = append(vmt.bootstrapOptions, opts...)
	return vmt
}

func (vmt vmTest) withContextOptions(opts ...fvm.Option) vmTest {
	vmt.contextOptions = append(vmt.contextOptions, opts...)
	return vmt
}

func createChainAndVm(chainID flow.ChainID) (flow.Chain, fvm.VM) {
	return chainID.Chain(), fvm.NewVirtualMachine()
}

func (vmt vmTest) run(
	f func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, view state.View),
) func(t *testing.T) {
	return func(t *testing.T) {
		baseOpts := []fvm.Option{
			// default chain is Testnet
			fvm.WithChain(flow.Testnet.Chain()),
		}

		opts := append(baseOpts, vmt.contextOptions...)
		ctx := fvm.NewContext(opts...)

		chain := ctx.Chain
		vm := fvm.NewVirtualMachine()

		view := delta.NewDeltaView(nil)

		baseBootstrapOpts := []fvm.BootstrapProcedureOption{
			fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
		}

		bootstrapOpts := append(baseBootstrapOpts, vmt.bootstrapOptions...)

		err := vm.Run(ctx, fvm.Bootstrap(unittest.ServiceAccountPublicKey, bootstrapOpts...), view)
		require.NoError(t, err)

		f(t, vm, chain, ctx, view)
	}
}

// bootstrapWith executes the bootstrap procedure and the custom bootstrap function
// and returns a prepared bootstrappedVmTest with all the state needed
func (vmt vmTest) bootstrapWith(
	bootstrap func(vm fvm.VM, chain flow.Chain, ctx fvm.Context, view state.View) error,
) (bootstrappedVmTest, error) {

	baseOpts := []fvm.Option{
		// default chain is Testnet
		fvm.WithChain(flow.Testnet.Chain()),
	}

	opts := append(baseOpts, vmt.contextOptions...)
	ctx := fvm.NewContext(opts...)

	chain := ctx.Chain
	vm := fvm.NewVirtualMachine()

	view := delta.NewDeltaView(nil)

	baseBootstrapOpts := []fvm.BootstrapProcedureOption{
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
	}

	bootstrapOpts := append(baseBootstrapOpts, vmt.bootstrapOptions...)

	err := vm.Run(ctx, fvm.Bootstrap(unittest.ServiceAccountPublicKey, bootstrapOpts...), view)
	if err != nil {
		return bootstrappedVmTest{}, err
	}

	err = bootstrap(vm, chain, ctx, view)
	if err != nil {
		return bootstrappedVmTest{}, err
	}

	return bootstrappedVmTest{chain, ctx, view}, nil
}

type bootstrappedVmTest struct {
	chain flow.Chain
	ctx   fvm.Context
	view  state.View
}

// run Runs a test from the bootstrapped state, without changing the bootstrapped state
func (vmt bootstrappedVmTest) run(
	f func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, view state.View),
) func(t *testing.T) {
	return func(t *testing.T) {
		f(t, fvm.NewVirtualMachine(), vmt.chain, vmt.ctx, vmt.view.NewChild())
	}
}

func TestHashing(t *testing.T) {

	t.Parallel()

	chain, vm := createChainAndVm(flow.Mainnet)

	ctx := fvm.NewContext(
		fvm.WithChain(chain),
		fvm.WithCadenceLogging(true),
	)

	ledger := testutil.RootBootstrappedLedger(vm, ctx)

	hashScript := func(hashName string) []byte {
		return []byte(fmt.Sprintf(
			`
				import Crypto

				pub fun main(data: [UInt8]): [UInt8] {
					return Crypto.hash(data, algorithm: HashAlgorithm.%s)
				}
			`, hashName))
	}
	hashWithTagScript := func(hashName string) []byte {
		return []byte(fmt.Sprintf(
			`
				import Crypto

				pub fun main(data: [UInt8], tag: String): [UInt8] {
					return Crypto.hashWithTag(data, tag: tag, algorithm: HashAlgorithm.%s)
				}
			`, hashName))
	}

	data := []byte("some random message")
	encodedBytes := make([]cadence.Value, len(data))
	for i := range encodedBytes {
		encodedBytes[i] = cadence.NewUInt8(data[i])
	}
	cadenceData := jsoncdc.MustEncode(cadence.NewArray(encodedBytes))

	// ===== Test Cases =====
	cases := []struct {
		Algo    runtime.HashAlgorithm
		WithTag bool
		Tag     string
		Check   func(t *testing.T, result string, scriptErr errors.CodedError, executionErr error)
	}{
		{
			Algo:    runtime.HashAlgorithmSHA2_256,
			WithTag: false,
			Check: func(t *testing.T, result string, scriptErr errors.CodedError, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "68fb87dfba69b956f4ba98b748a75a604f99b38a4f2740290037957f7e830da8", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmSHA2_384,
			WithTag: false,
			Check: func(t *testing.T, result string, scriptErr errors.CodedError, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "a9b3e62ab9b2a33020e015f245b82e063afd1398211326408bc8fc31c2c15859594b0aee263fbb02f6d8b5065ad49df2", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmSHA3_256,
			WithTag: false,
			Check: func(t *testing.T, result string, scriptErr errors.CodedError, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "38effea5ab9082a2cb0dc9adfafaf88523e8f3ce74bfbeac85ffc719cc2c4677", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmSHA3_384,
			WithTag: false,
			Check: func(t *testing.T, result string, scriptErr errors.CodedError, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "f41e8de9af0c1f46fc56d5a776f1bd500530879a85f3b904821810295927e13a54f3e936dddb84669021052eb12966c3", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmKECCAK_256,
			WithTag: false,
			Check: func(t *testing.T, result string, scriptErr errors.CodedError, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "1d5ced4738dd4e0bb4628dad7a7b59b8e339a75ece97a4ad004773a49ed7b5bc", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmKECCAK_256,
			WithTag: true,
			Tag:     "some_tag",
			Check: func(t *testing.T, result string, scriptErr errors.CodedError, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "8454ec77f76b229a473770c91e3ea6e7e852416d747805215d15d53bdc56ce5f", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmSHA2_256,
			WithTag: true,
			Tag:     "some_tag",
			Check: func(t *testing.T, result string, scriptErr errors.CodedError, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "4e07609b9a856a5e10703d1dba73be34d9ca0f4e780859d66983f41d746ec8b2", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmSHA2_384,
			WithTag: true,
			Tag:     "some_tag",
			Check: func(t *testing.T, result string, scriptErr errors.CodedError, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "f9bd89e15f341a225656944dc8b3c405e66a0f97838ad44c9803164c911e677aea7ad4e24486fba3f803d83ed1ccfce5", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmSHA3_256,
			WithTag: true,
			Tag:     "some_tag",
			Check: func(t *testing.T, result string, scriptErr errors.CodedError, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "f59e2ccc9d7f008a96948a31573670d9976a4a161601ab1cd1d2da019779a0f6", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmSHA3_384,
			WithTag: true,
			Tag:     "some_tag",
			Check: func(t *testing.T, result string, scriptErr errors.CodedError, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "e7875eafdb53327faeace8478d1650c6547d04fb4fb42f34509ad64bde0267bea7e1b3af8fda3ef9d9c9327dd4e97a96", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmKMAC128_BLS_BLS12_381,
			WithTag: false,
			Check: func(t *testing.T, result string, scriptErr errors.CodedError, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "44dc46111abacfe2bb4a04cea4805aad03f84e4849f138cc3ed431478472b185548628e96d0c963b21ebaf17132d73fc13031eb82d5f4cbe3b6047ff54d20e8d663904373d73348b97ce18305ebc56114cb7e7394e486684007f78aa59abc5d0a8f6bae6bd186db32528af80857cd12112ce6960be29c96074df9c4aaed5b0e6", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmKMAC128_BLS_BLS12_381,
			WithTag: true,
			Tag:     "some_tag",
			Check: func(t *testing.T, result string, scriptErr errors.CodedError, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "de7d9aa24274fa12c98cce5c09eea0634108ead2e91828b9a9a450e878088393e3e63eb4b19834f579ce215b00a9915919b67a71dab1112560319e6e1e5e9ad0fb670e8a09d586508c84547cee7ddbe8c9362c996846154865eb271bdc4523dbcdbdae5a77391fb54374f37534c8bb2281589cb2e3d62742596cdad7e4f9f35c", result)
			},
		},
	}
	// ======================

	for i, c := range cases {
		t.Run(fmt.Sprintf("case %d: %s with tag: %v", i, c.Algo, c.WithTag), func(t *testing.T) {
			code := hashScript(c.Algo.Name())
			if c.WithTag {
				code = hashWithTagScript(c.Algo.Name())
			}

			script := fvm.Script(code)

			if c.WithTag {
				script = script.WithArguments(
					cadenceData,
					jsoncdc.MustEncode(cadence.String(c.Tag)),
				)
			} else {
				script = script.WithArguments(
					cadenceData,
				)
			}

			err := vm.Run(ctx, script, ledger)

			byteResult := make([]byte, 0)
			if err == nil && script.Err == nil {
				cadenceArray := script.Value.(cadence.Array)
				for _, value := range cadenceArray.Values {
					byteResult = append(byteResult, value.(cadence.UInt8).ToGoValue().(uint8))
				}
			}

			c.Check(t, hex.EncodeToString(byteResult), script.Err, err)
		})
	}

	hashAlgos := []runtime.HashAlgorithm{
		runtime.HashAlgorithmSHA2_256,
		runtime.HashAlgorithmSHA3_256,
		runtime.HashAlgorithmSHA2_384,
		runtime.HashAlgorithmSHA3_384,
		runtime.HashAlgorithmKMAC128_BLS_BLS12_381,
		runtime.HashAlgorithmKECCAK_256,
	}

	for i, algo := range hashAlgos {
		t.Run(fmt.Sprintf("compare hash results without tag %v: %v", i, algo), func(t *testing.T) {
			code := hashWithTagScript(algo.Name())
			script := fvm.Script(code)
			script = script.WithArguments(
				cadenceData,
				jsoncdc.MustEncode(cadence.String("")),
			)
			err := vm.Run(ctx, script, ledger)
			require.NoError(t, err)
			require.NoError(t, script.Err)

			result1 := make([]byte, 0)
			cadenceArray := script.Value.(cadence.Array)
			for _, value := range cadenceArray.Values {
				result1 = append(result1, value.(cadence.UInt8).ToGoValue().(uint8))
			}

			code = hashScript(algo.Name())
			script = fvm.Script(code)
			script = script.WithArguments(
				cadenceData,
			)
			err = vm.Run(ctx, script, ledger)
			require.NoError(t, err)
			require.NoError(t, script.Err)

			result2 := make([]byte, 0)
			cadenceArray = script.Value.(cadence.Array)
			for _, value := range cadenceArray.Values {
				result2 = append(result2, value.(cadence.UInt8).ToGoValue().(uint8))
			}

			result3, err := fvmCrypto.HashWithTag(fvmCrypto.RuntimeToCryptoHashingAlgorithm(algo), "", data)
			require.NoError(t, err)

			require.Equal(t, result1, result2)
			require.Equal(t, result1, result3)
		})
	}
}

func TestWithServiceAccount(t *testing.T) {

	t.Parallel()

	chain, vm := createChainAndVm(flow.Mainnet)

	ctxA := fvm.NewContext(
		fvm.WithChain(chain),
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
	)

	view := delta.NewDeltaView(nil)

	txBody := flow.NewTransactionBody().
		SetScript([]byte(`transaction { prepare(signer: AuthAccount) { AuthAccount(payer: signer) } }`)).
		AddAuthorizer(chain.ServiceAddress())

	t.Run("With service account enabled", func(t *testing.T) {
		tx := fvm.Transaction(txBody, 0)

		err := vm.Run(ctxA, tx, view)
		require.NoError(t, err)

		// transaction should fail on non-bootstrapped ledger
		require.Error(t, tx.Err)
	})

	t.Run("With service account disabled", func(t *testing.T) {
		ctxB := fvm.NewContextFromParent(
			ctxA,
			fvm.WithServiceAccount(false))

		tx := fvm.Transaction(txBody, 0)

		err := vm.Run(ctxB, tx, view)
		require.NoError(t, err)

		// transaction should succeed on non-bootstrapped ledger
		assert.NoError(t, tx.Err)
	})
}

func TestEventLimits(t *testing.T) {
	chain, vm := createChainAndVm(flow.Mainnet)

	ctx := fvm.NewContext(
		fvm.WithChain(chain),
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
	)

	ledger := testutil.RootBootstrappedLedger(vm, ctx)

	testContract := `
	access(all) contract TestContract {
		access(all) event LargeEvent(value: Int256, str: String, list: [UInt256], dic: {String: String})
		access(all) fun EmitEvent() {
			var s: Int256 = 1024102410241024
			var i = 0

			while i < 20 {
				emit LargeEvent(value: s, str: s.toString(), list:[], dic:{s.toString():s.toString()})
				i = i + 1
			}
		}
	}
	`

	deployingContractScriptTemplate := `
		transaction {
			prepare(signer: AuthAccount) {
				let code = "%s".decodeHex()
				signer.contracts.add(
					name: "TestContract",
					code: code
				)
		}
	}
	`

	ctx = fvm.NewContextFromParent(
		ctx,
		fvm.WithEventCollectionSizeLimit(2))

	txBody := flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(deployingContractScriptTemplate, hex.EncodeToString([]byte(testContract))))).
		SetPayer(chain.ServiceAddress()).
		AddAuthorizer(chain.ServiceAddress())

	tx := fvm.Transaction(txBody, 0)
	err := vm.Run(ctx, tx, ledger)
	require.NoError(t, err)

	txBody = flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(`
		import TestContract from 0x%s
			transaction {
			prepare(acct: AuthAccount) {}
			execute {
				TestContract.EmitEvent()
			}
		}`, chain.ServiceAddress()))).
		AddAuthorizer(chain.ServiceAddress())

	t.Run("With limits", func(t *testing.T) {
		txBody.Payer = unittest.RandomAddressFixture()
		tx := fvm.Transaction(txBody, 0)
		err := vm.Run(ctx, tx, ledger)
		require.NoError(t, err)

		// transaction should fail due to event size limit
		assert.Error(t, tx.Err)
	})

	t.Run("With service account as payer", func(t *testing.T) {
		txBody.Payer = chain.ServiceAddress()
		tx := fvm.Transaction(txBody, 0)
		err := vm.Run(ctx, tx, ledger)
		require.NoError(t, err)

		unittest.EnsureEventsIndexSeq(t, tx.Events, chain.ChainID())

		// transaction should not fail due to event size limit
		assert.NoError(t, tx.Err)
	})
}

// TestHappyPathSigning checks that a signing a transaction with `Sign` doesn't produce an error.
// Transaction verification tests are in `TestVerifySignatureFromTransaction`.
func TestHappyPathTransactionSigning(t *testing.T) {

	newVMTest().run(
		func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, view state.View) {
			// Create an account private key.
			privateKey, err := testutil.GenerateAccountPrivateKey()
			require.NoError(t, err)

			// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
			accounts, err := testutil.CreateAccounts(vm, view, []flow.AccountPrivateKey{privateKey}, chain)
			require.NoError(t, err)

			txBody := flow.NewTransactionBody().
				SetScript([]byte(`transaction(){}`))

			txBody.SetProposalKey(accounts[0], 0, 0)
			txBody.SetPayer(accounts[0])

			hasher, err := exeUtils.NewHasher(privateKey.HashAlgo)
			require.NoError(t, err)

			sig, err := txBody.Sign(txBody.EnvelopeMessage(), privateKey.PrivateKey, hasher)
			require.NoError(t, err)
			txBody.AddEnvelopeSignature(accounts[0], 0, sig)

			tx := fvm.Transaction(txBody, 0)

			err = vm.Run(ctx, tx, view)
			require.NoError(t, err)
			require.NoError(t, tx.Err)
		},
	)
}

func TestTransactionFeeDeduction(t *testing.T) {
	getBalance := func(vm fvm.VM, chain flow.Chain, ctx fvm.Context, view state.View, address flow.Address) uint64 {

		code := []byte(fmt.Sprintf(`
					import FungibleToken from 0x%s
					import FlowToken from 0x%s

					pub fun main(account: Address): UFix64 {
						let acct = getAccount(account)
						let vaultRef = acct.getCapability(/public/flowTokenBalance)
							.borrow<&FlowToken.Vault{FungibleToken.Balance}>()
							?? panic("Could not borrow Balance reference to the Vault")

						return vaultRef.balance
					}
				`, fvm.FungibleTokenAddress(chain), fvm.FlowTokenAddress(chain)))
		script := fvm.Script(code).WithArguments(
			jsoncdc.MustEncode(cadence.NewAddress(address)),
		)

		err := vm.Run(ctx, script, view)
		require.NoError(t, err)
		require.NoError(t, script.Err)
		return script.Value.ToGoValue().(uint64)
	}

	type testCase struct {
		name          string
		fundWith      uint64
		tryToTransfer uint64
		gasLimit      uint64
		checkResult   func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure)
	}

	txFees := uint64(1_000)              // 0.00001
	fundingAmount := uint64(100_000_000) // 1.0
	transferAmount := uint64(123_456)
	minimumStorageReservation := fvm.DefaultMinimumStorageReservation.ToGoValue().(uint64)

	testCases := []testCase{
		{
			name:          "Transaction fees are deducted",
			fundWith:      fundingAmount,
			tryToTransfer: 0,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Equal(t, txFees, balanceBefore-balanceAfter)
			},
		},
		{
			name:          "Transaction fee deduction emits events",
			fundWith:      fundingAmount,
			tryToTransfer: 0,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)

				var deposits []flow.Event
				var withdraws []flow.Event

				chain := flow.Testnet.Chain()
				for _, e := range tx.Events {
					if string(e.Type) == fmt.Sprintf("A.%s.FlowToken.TokensDeposited", fvm.FlowTokenAddress(chain)) {
						deposits = append(deposits, e)
					}
					if string(e.Type) == fmt.Sprintf("A.%s.FlowToken.TokensWithdrawn", fvm.FlowTokenAddress(chain)) {
						withdraws = append(withdraws, e)
					}
				}

				unittest.EnsureEventsIndexSeq(t, tx.Events, chain.ChainID())
				require.Len(t, deposits, 2)
				require.Len(t, withdraws, 2)
			},
		},
		{
			name:          "Transaction fees are deducted and tx is applied",
			fundWith:      fundingAmount,
			tryToTransfer: transferAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Equal(t, txFees+transferAmount, balanceBefore-balanceAfter)
			},
		},
		{
			name:          "Transaction fees are deducted and fee deduction is emitted",
			fundWith:      fundingAmount,
			tryToTransfer: transferAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				chain := flow.Testnet.Chain()

				var feeDeduction flow.Event // fee deduction event
				for _, e := range tx.Events {
					if string(e.Type) == fmt.Sprintf("A.%s.FlowFees.FeesDeducted", environment.FlowFeesAddress(chain)) {
						feeDeduction = e
						break
					}
				}
				unittest.EnsureEventsIndexSeq(t, tx.Events, chain.ChainID())
				require.NotEmpty(t, feeDeduction.Payload)

				payload, err := jsoncdc.Decode(nil, feeDeduction.Payload)
				require.NoError(t, err)

				event := payload.(cadence.Event)

				require.Equal(t, txFees, event.Fields[0].ToGoValue())
				// Inclusion effort should be equivalent to 1.0 UFix64
				require.Equal(t, uint64(100_000_000), event.Fields[1].ToGoValue())
				// Execution effort should be non-0
				require.Greater(t, event.Fields[2].ToGoValue(), uint64(0))

			},
		},
		{
			name:          "If just enough balance, fees are deducted",
			fundWith:      txFees + transferAmount,
			tryToTransfer: transferAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Equal(t, uint64(0), balanceAfter)
			},
		},
		{
			// this is an edge case that is not applicable to any network.
			// If storage limits were on this would fail due to storage limits
			name:          "If not enough balance, transaction succeeds and fees are deducted to 0",
			fundWith:      txFees,
			tryToTransfer: 1,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Equal(t, uint64(0), balanceAfter)
			},
		},
		{
			name:          "If tx fails, fees are deducted",
			fundWith:      fundingAmount,
			tryToTransfer: 2 * fundingAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.Error(t, tx.Err)
				require.Equal(t, fundingAmount-txFees, balanceAfter)
			},
		},
		{
			name:          "If tx fails, fee deduction events are emitted",
			fundWith:      fundingAmount,
			tryToTransfer: 2 * fundingAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.Error(t, tx.Err)

				var deposits []flow.Event
				var withdraws []flow.Event

				chain := flow.Testnet.Chain()

				for _, e := range tx.Events {
					if string(e.Type) == fmt.Sprintf("A.%s.FlowToken.TokensDeposited", fvm.FlowTokenAddress(chain)) {
						deposits = append(deposits, e)
					}
					if string(e.Type) == fmt.Sprintf("A.%s.FlowToken.TokensWithdrawn", fvm.FlowTokenAddress(chain)) {
						withdraws = append(withdraws, e)
					}
				}

				unittest.EnsureEventsIndexSeq(t, tx.Events, chain.ChainID())
				require.Len(t, deposits, 1)
				require.Len(t, withdraws, 1)
			},
		},
		{
			name:          "If tx fails because of gas limit reached, fee deduction events are emitted",
			fundWith:      txFees + transferAmount,
			tryToTransfer: transferAmount,
			gasLimit:      uint64(2),
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.ErrorContains(t, tx.Err, "computation exceeds limit (2)")

				var deposits []flow.Event
				var withdraws []flow.Event

				chain := flow.Testnet.Chain()

				for _, e := range tx.Events {
					if string(e.Type) == fmt.Sprintf("A.%s.FlowToken.TokensDeposited", fvm.FlowTokenAddress(chain)) {
						deposits = append(deposits, e)
					}
					if string(e.Type) == fmt.Sprintf("A.%s.FlowToken.TokensWithdrawn", fvm.FlowTokenAddress(chain)) {
						withdraws = append(withdraws, e)
					}
				}

				unittest.EnsureEventsIndexSeq(t, tx.Events, chain.ChainID())
				require.Len(t, deposits, 1)
				require.Len(t, withdraws, 1)
			},
		},
	}

	testCasesWithStorageEnabled := []testCase{
		{
			name:          "Transaction fees are deducted",
			fundWith:      fundingAmount,
			tryToTransfer: 0,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Equal(t, txFees, balanceBefore-balanceAfter)
			},
		},
		{
			name:          "Transaction fee deduction emits events",
			fundWith:      fundingAmount,
			tryToTransfer: 0,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)

				var deposits []flow.Event
				var withdraws []flow.Event

				chain := flow.Testnet.Chain()

				for _, e := range tx.Events {
					if string(e.Type) == fmt.Sprintf("A.%s.FlowToken.TokensDeposited", fvm.FlowTokenAddress(chain)) {
						deposits = append(deposits, e)
					}
					if string(e.Type) == fmt.Sprintf("A.%s.FlowToken.TokensWithdrawn", fvm.FlowTokenAddress(chain)) {
						withdraws = append(withdraws, e)
					}
				}

				unittest.EnsureEventsIndexSeq(t, tx.Events, chain.ChainID())
				require.Len(t, deposits, 2)
				require.Len(t, withdraws, 2)
			},
		},
		{
			name:          "Transaction fees are deducted and tx is applied",
			fundWith:      fundingAmount,
			tryToTransfer: transferAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Equal(t, txFees+transferAmount, balanceBefore-balanceAfter)
			},
		},
		{
			name:          "If just enough balance, fees are deducted",
			fundWith:      txFees + transferAmount,
			tryToTransfer: transferAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Equal(t, minimumStorageReservation, balanceAfter)
			},
		},
		{
			name:          "If tx fails, fees are deducted",
			fundWith:      fundingAmount,
			tryToTransfer: 2 * fundingAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.Error(t, tx.Err)
				require.Equal(t, fundingAmount-txFees+minimumStorageReservation, balanceAfter)
			},
		},
		{
			name:          "If tx fails, fee deduction events are emitted",
			fundWith:      fundingAmount,
			tryToTransfer: 2 * fundingAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.Error(t, tx.Err)

				var deposits []flow.Event
				var withdraws []flow.Event

				chain := flow.Testnet.Chain()

				for _, e := range tx.Events {
					if string(e.Type) == fmt.Sprintf("A.%s.FlowToken.TokensDeposited", fvm.FlowTokenAddress(chain)) {
						deposits = append(deposits, e)
					}
					if string(e.Type) == fmt.Sprintf("A.%s.FlowToken.TokensWithdrawn", fvm.FlowTokenAddress(chain)) {
						withdraws = append(withdraws, e)
					}
				}

				unittest.EnsureEventsIndexSeq(t, tx.Events, chain.ChainID())
				require.Len(t, deposits, 1)
				require.Len(t, withdraws, 1)
			},
		},
		{
			name:          "If balance at minimum, transaction fails, fees are deducted and fee deduction events are emitted",
			fundWith:      0,
			tryToTransfer: 0,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.Error(t, tx.Err)
				require.Equal(t, minimumStorageReservation-txFees, balanceAfter)

				var deposits []flow.Event
				var withdraws []flow.Event

				chain := flow.Testnet.Chain()

				for _, e := range tx.Events {
					if string(e.Type) == fmt.Sprintf("A.%s.FlowToken.TokensDeposited", fvm.FlowTokenAddress(chain)) {
						deposits = append(deposits, e)
					}
					if string(e.Type) == fmt.Sprintf("A.%s.FlowToken.TokensWithdrawn", fvm.FlowTokenAddress(chain)) {
						withdraws = append(withdraws, e)
					}
				}

				unittest.EnsureEventsIndexSeq(t, tx.Events, chain.ChainID())
				require.Len(t, deposits, 1)
				require.Len(t, withdraws, 1)
			},
		},
	}

	runTx := func(tc testCase) func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, view state.View) {
		return func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, view state.View) {
			// ==== Create an account ====
			privateKey, txBody := testutil.CreateAccountCreationTransaction(t, chain)

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			tx := fvm.Transaction(txBody, 0)

			err = vm.Run(ctx, tx, view)
			require.NoError(t, err)

			assert.NoError(t, tx.Err)

			assert.Len(t, tx.Events, 10)
			unittest.EnsureEventsIndexSeq(t, tx.Events, chain.ChainID())

			accountCreatedEvents := filterAccountCreatedEvents(tx.Events)

			require.Len(t, accountCreatedEvents, 1)

			// read the address of the account created (e.g. "0x01" and convert it to flow.address)
			data, err := jsoncdc.Decode(nil, accountCreatedEvents[0].Payload)
			require.NoError(t, err)
			address := flow.ConvertAddress(
				data.(cadence.Event).Fields[0].(cadence.Address))

			// ==== Transfer tokens to new account ====
			txBody = transferTokensTx(chain).
				AddAuthorizer(chain.ServiceAddress()).
				AddArgument(jsoncdc.MustEncode(cadence.UFix64(tc.fundWith))).
				AddArgument(jsoncdc.MustEncode(cadence.NewAddress(address)))

			txBody.SetProposalKey(chain.ServiceAddress(), 0, 1)
			txBody.SetPayer(chain.ServiceAddress())

			err = testutil.SignEnvelope(
				txBody,
				chain.ServiceAddress(),
				unittest.ServiceAccountPrivateKey,
			)
			require.NoError(t, err)

			tx = fvm.Transaction(txBody, 0)

			err = vm.Run(ctx, tx, view)
			require.NoError(t, err)
			require.NoError(t, tx.Err)

			balanceBefore := getBalance(vm, chain, ctx, view, address)

			// ==== Transfer tokens from new account ====

			txBody = transferTokensTx(chain).
				AddAuthorizer(address).
				AddArgument(jsoncdc.MustEncode(cadence.UFix64(tc.tryToTransfer))).
				AddArgument(jsoncdc.MustEncode(cadence.NewAddress(chain.ServiceAddress())))

			txBody.SetProposalKey(address, 0, 0)
			txBody.SetPayer(address)

			if tc.gasLimit == 0 {
				txBody.SetGasLimit(fvm.DefaultComputationLimit)
			} else {
				txBody.SetGasLimit(tc.gasLimit)
			}

			err = testutil.SignEnvelope(
				txBody,
				address,
				privateKey,
			)
			require.NoError(t, err)

			tx = fvm.Transaction(txBody, 0)

			err = vm.Run(ctx, tx, view)
			require.NoError(t, err)

			balanceAfter := getBalance(vm, chain, ctx, view, address)

			tc.checkResult(
				t,
				balanceBefore,
				balanceAfter,
				tx,
			)
		}
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Transaction Fees %d: %s", i, tc.name), newVMTest().withBootstrapProcedureOptions(
			fvm.WithTransactionFee(fvm.DefaultTransactionFees),
			fvm.WithExecutionMemoryLimit(math.MaxUint64),
			fvm.WithExecutionEffortWeights(mainnetExecutionEffortWeights),
			fvm.WithExecutionMemoryWeights(meter.DefaultMemoryWeights),
		).withContextOptions(
			fvm.WithTransactionFeesEnabled(true),
		).run(
			runTx(tc)),
		)
	}

	for i, tc := range testCasesWithStorageEnabled {
		t.Run(fmt.Sprintf("Transaction Fees with storage %d: %s", i, tc.name), newVMTest().withBootstrapProcedureOptions(
			fvm.WithTransactionFee(fvm.DefaultTransactionFees),
			fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
			fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
			fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
			fvm.WithExecutionMemoryLimit(math.MaxUint64),
			fvm.WithExecutionEffortWeights(mainnetExecutionEffortWeights),
			fvm.WithExecutionMemoryWeights(meter.DefaultMemoryWeights),
		).withContextOptions(
			fvm.WithTransactionFeesEnabled(true),
			fvm.WithAccountStorageLimit(true),
		).run(
			runTx(tc)),
		)
	}
}

func TestSettingExecutionWeights(t *testing.T) {

	t.Run("transaction should fail with high weights", newVMTest().withBootstrapProcedureOptions(
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
		fvm.WithExecutionEffortWeights(
			meter.ExecutionEffortWeights{
				common.ComputationKindLoop: 100_000 << meter.MeterExecutionInternalPrecisionBytes,
			},
		),
	).run(
		func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, view state.View) {

			txBody := flow.NewTransactionBody().
				SetScript([]byte(`
				transaction {
                  prepare(signer: AuthAccount) {
					var a = 0
					while a < 100 {
						a = a + 1
					}
                  }
                }
			`)).
				SetProposalKey(chain.ServiceAddress(), 0, 0).
				AddAuthorizer(chain.ServiceAddress()).
				SetPayer(chain.ServiceAddress())

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			tx := fvm.Transaction(txBody, 0)
			err = vm.Run(ctx, tx, view)
			require.NoError(t, err)

			assert.True(t, errors.IsComputationLimitExceededError(tx.Err))
		},
	))

	memoryWeights := make(map[common.MemoryKind]uint64)
	for k, v := range meter.DefaultMemoryWeights {
		memoryWeights[k] = v
	}

	const highWeight = 20_000_000_000
	memoryWeights[common.MemoryKindIntegerExpression] = highWeight

	t.Run("normal transactions should fail with high memory weights", newVMTest().withBootstrapProcedureOptions(
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
		fvm.WithExecutionMemoryWeights(
			memoryWeights,
		),
	).withContextOptions(
		fvm.WithMemoryLimit(10_000_000_000),
	).run(
		func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, view state.View) {

			// Create an account private key.
			privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
			require.NoError(t, err)

			// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
			accounts, err := testutil.CreateAccounts(vm, view, privateKeys, chain)
			require.NoError(t, err)

			txBody := flow.NewTransactionBody().
				SetScript([]byte(`
				transaction {
                  prepare(signer: AuthAccount) {
					var a = 1
                  }
                }
			`)).
				SetProposalKey(accounts[0], 0, 0).
				AddAuthorizer(accounts[0]).
				SetPayer(accounts[0])

			err = testutil.SignTransaction(txBody, accounts[0], privateKeys[0], 0)
			require.NoError(t, err)

			tx := fvm.Transaction(txBody, 0)
			err = vm.Run(ctx, tx, view)
			require.NoError(t, err)
			require.Greater(t, tx.MemoryEstimate, uint64(highWeight))

			assert.True(t, errors.IsMemoryLimitExceededError(tx.Err))
		},
	))

	t.Run("service account transactions should not fail with high memory weights", newVMTest().withBootstrapProcedureOptions(
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
		fvm.WithExecutionMemoryWeights(
			memoryWeights,
		),
	).withContextOptions(
		fvm.WithMemoryLimit(10_000_000_000),
	).run(
		func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, view state.View) {

			txBody := flow.NewTransactionBody().
				SetScript([]byte(`
				transaction {
                  prepare(signer: AuthAccount) {
					var a = 1
                  }
                }
			`)).
				SetProposalKey(chain.ServiceAddress(), 0, 0).
				AddAuthorizer(chain.ServiceAddress()).
				SetPayer(chain.ServiceAddress())

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			tx := fvm.Transaction(txBody, 0)
			err = vm.Run(ctx, tx, view)
			require.NoError(t, err)
			require.Greater(t, tx.MemoryEstimate, uint64(highWeight))

			require.NoError(t, tx.Err)
		},
	))

	memoryWeights = make(map[common.MemoryKind]uint64)
	for k, v := range meter.DefaultMemoryWeights {
		memoryWeights[k] = v
	}
	memoryWeights[common.MemoryKindBreakStatement] = 1_000_000
	t.Run("transaction should fail with low memory limit (set in the state)", newVMTest().withBootstrapProcedureOptions(
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
		fvm.WithExecutionMemoryLimit(
			100_000_000,
		),
		fvm.WithExecutionMemoryWeights(
			memoryWeights,
		),
	).run(
		func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, view state.View) {
			privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
			require.NoError(t, err)

			accounts, err := testutil.CreateAccounts(vm, view, privateKeys, chain)
			require.NoError(t, err)

			// This transaction is specially designed to use a lot of breaks
			// as the weight for breaks is much higher than usual.
			// putting a `while true {break}` in a loop does not use the same amount of memory.
			txBody := flow.NewTransactionBody().
				SetScript([]byte(`
				transaction {
					prepare(signer: AuthAccount) {
						while true {break};while true {break};while true {break};while true {break};while true {break};
						while true {break};while true {break};while true {break};while true {break};while true {break};
						while true {break};while true {break};while true {break};while true {break};while true {break};
						while true {break};while true {break};while true {break};while true {break};while true {break};
						while true {break};while true {break};while true {break};while true {break};while true {break};
						while true {break};while true {break};while true {break};while true {break};while true {break};
						while true {break};while true {break};while true {break};while true {break};while true {break};
						while true {break};while true {break};while true {break};while true {break};while true {break};
						while true {break};while true {break};while true {break};while true {break};while true {break};
						while true {break};while true {break};while true {break};while true {break};while true {break};
						while true {break};while true {break};while true {break};while true {break};while true {break};
						while true {break};while true {break};while true {break};while true {break};while true {break};
						while true {break};while true {break};while true {break};while true {break};while true {break};
						while true {break};while true {break};while true {break};while true {break};while true {break};
						while true {break};while true {break};while true {break};while true {break};while true {break};
						while true {break};while true {break};while true {break};while true {break};while true {break};
						while true {break};while true {break};while true {break};while true {break};while true {break};
						while true {break};while true {break};while true {break};while true {break};while true {break};
						while true {break};while true {break};while true {break};while true {break};while true {break};
						while true {break};while true {break};while true {break};while true {break};while true {break};
					}
				}
			`))

			err = testutil.SignTransaction(txBody, accounts[0], privateKeys[0], 0)
			require.NoError(t, err)

			tx := fvm.Transaction(txBody, 0)
			err = vm.Run(ctx, tx, view)
			require.NoError(t, err)
			// There are 100 breaks and each break uses 1_000_000 memory
			require.Greater(t, tx.MemoryEstimate, uint64(100_000_000))

			assert.True(t, errors.IsMemoryLimitExceededError(tx.Err))
		},
	))

	t.Run("transaction should fail if create account weight is high", newVMTest().withBootstrapProcedureOptions(
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
		fvm.WithExecutionEffortWeights(
			meter.ExecutionEffortWeights{
				environment.ComputationKindCreateAccount: (fvm.DefaultComputationLimit + 1) << meter.MeterExecutionInternalPrecisionBytes,
			},
		),
	).run(
		func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, view state.View) {
			txBody := flow.NewTransactionBody().
				SetScript([]byte(`
				transaction {
                  prepare(signer: AuthAccount) {
					AuthAccount(payer: signer)
                  }
                }
			`)).
				SetProposalKey(chain.ServiceAddress(), 0, 0).
				AddAuthorizer(chain.ServiceAddress()).
				SetPayer(chain.ServiceAddress())

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			tx := fvm.Transaction(txBody, 0)
			err = vm.Run(ctx, tx, view)
			require.NoError(t, err)

			assert.True(t, errors.IsComputationLimitExceededError(tx.Err))
		},
	))

	t.Run("transaction should fail if create account weight is high", newVMTest().withBootstrapProcedureOptions(
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
		fvm.WithExecutionEffortWeights(
			meter.ExecutionEffortWeights{
				environment.ComputationKindCreateAccount: 100_000_000 << meter.MeterExecutionInternalPrecisionBytes,
			},
		),
	).run(
		func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, view state.View) {

			txBody := flow.NewTransactionBody().
				SetScript([]byte(`
				transaction {
                  prepare(signer: AuthAccount) {
					AuthAccount(payer: signer)
                  }
                }
			`)).
				SetProposalKey(chain.ServiceAddress(), 0, 0).
				AddAuthorizer(chain.ServiceAddress()).
				SetPayer(chain.ServiceAddress())

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			tx := fvm.Transaction(txBody, 0)
			err = vm.Run(ctx, tx, view)
			require.NoError(t, err)

			assert.True(t, errors.IsComputationLimitExceededError(tx.Err))
		},
	))

	t.Run("transaction should fail if create account weight is high", newVMTest().withBootstrapProcedureOptions(
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
		fvm.WithExecutionEffortWeights(
			meter.ExecutionEffortWeights{
				environment.ComputationKindCreateAccount: 100_000_000 << meter.MeterExecutionInternalPrecisionBytes,
			},
		),
	).run(
		func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, view state.View) {
			txBody := flow.NewTransactionBody().
				SetScript([]byte(`
				transaction {
                  prepare(signer: AuthAccount) {
					AuthAccount(payer: signer)
                  }
                }
			`)).
				SetProposalKey(chain.ServiceAddress(), 0, 0).
				AddAuthorizer(chain.ServiceAddress()).
				SetPayer(chain.ServiceAddress())

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			tx := fvm.Transaction(txBody, 0)
			err = vm.Run(ctx, tx, view)
			require.NoError(t, err)

			assert.True(t, errors.IsComputationLimitExceededError(tx.Err))
		},
	))

	t.Run("transaction should not use up more computation that the transaction body itself", newVMTest().withBootstrapProcedureOptions(
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
		fvm.WithTransactionFee(fvm.DefaultTransactionFees),
		fvm.WithExecutionEffortWeights(
			meter.ExecutionEffortWeights{
				common.ComputationKindStatement:          0,
				common.ComputationKindLoop:               1 << meter.MeterExecutionInternalPrecisionBytes,
				common.ComputationKindFunctionInvocation: 0,
			},
		),
	).withContextOptions(
		fvm.WithAccountStorageLimit(true),
		fvm.WithTransactionFeesEnabled(true),
		fvm.WithMemoryLimit(math.MaxUint64),
	).run(
		func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, view state.View) {
			// Use the maximum amount of computation so that the transaction still passes.
			loops := uint64(997)
			maxExecutionEffort := uint64(997)
			txBody := flow.NewTransactionBody().
				SetScript([]byte(fmt.Sprintf(`
				transaction() {prepare(signer: AuthAccount){var i=0;  while i < %d {i = i +1 } } execute{}}
			`, loops))).
				SetProposalKey(chain.ServiceAddress(), 0, 0).
				AddAuthorizer(chain.ServiceAddress()).
				SetPayer(chain.ServiceAddress()).
				SetGasLimit(maxExecutionEffort)

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			tx := fvm.Transaction(txBody, 0)
			err = vm.Run(ctx, tx, view)
			require.NoError(t, err)
			require.NoError(t, tx.Err)

			// expected used is number of loops.
			assert.Equal(t, loops, tx.ComputationUsed)

			// increasing the number of loops should fail the transaction.
			loops = loops + 1
			txBody = flow.NewTransactionBody().
				SetScript([]byte(fmt.Sprintf(`
				transaction() {prepare(signer: AuthAccount){var i=0;  while i < %d {i = i +1 } } execute{}}
			`, loops))).
				SetProposalKey(chain.ServiceAddress(), 0, 1).
				AddAuthorizer(chain.ServiceAddress()).
				SetPayer(chain.ServiceAddress()).
				SetGasLimit(maxExecutionEffort)

			err = testutil.SignTransactionAsServiceAccount(txBody, 1, chain)
			require.NoError(t, err)

			tx = fvm.Transaction(txBody, 0)
			err = vm.Run(ctx, tx, view)
			require.NoError(t, err)

			require.ErrorContains(t, tx.Err, "computation exceeds limit (997)")
			// computation used should the actual computation used.
			assert.Equal(t, loops, tx.ComputationUsed)

			for _, event := range tx.Events {
				// the fee deduction event should only contain the max gas worth of execution effort.
				if strings.Contains(string(event.Type), "FlowFees.FeesDeducted") {
					ev, err := jsoncdc.Decode(nil, event.Payload)
					require.NoError(t, err)
					assert.Equal(t, maxExecutionEffort, ev.(cadence.Event).Fields[2].ToGoValue().(uint64))
				}
			}
			unittest.EnsureEventsIndexSeq(t, tx.Events, chain.ChainID())
		},
	))
}

func TestStorageUsed(t *testing.T) {
	t.Parallel()

	chain, vm := createChainAndVm(flow.Testnet)

	ctx := fvm.NewContext(
		fvm.WithChain(chain),
		fvm.WithCadenceLogging(true),
	)

	code := []byte(`
        pub fun main(): UInt64 {

            var addresses: [Address]= [
                0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731,
                0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731,
                0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731,
                0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731,
                0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731,
                0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731,
                0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731,
                0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731,
                0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731,
                0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731,
                0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731
            ]

            var storageUsed: UInt64 = 0
            for address in addresses {
                let account = getAccount(address)
                storageUsed = account.storageUsed
            }

            return storageUsed
        }
	`)

	address, err := hex.DecodeString("2a3c4c2581cef731")
	require.NoError(t, err)

	accountStatusId := flow.AccountStatusRegisterID(
		flow.BytesToAddress(address))

	simpleView := delta.NewDeltaView(nil)
	status := environment.NewAccountStatus()
	status.SetStorageUsed(5)
	err = simpleView.Set(accountStatusId, status.ToBytes())
	require.NoError(t, err)

	script := fvm.Script(code)

	err = vm.Run(ctx, script, simpleView)
	require.NoError(t, err)

	assert.Equal(t, cadence.NewUInt64(5), script.Value)
}

func TestEnforcingComputationLimit(t *testing.T) {
	t.Parallel()

	chain, vm := createChainAndVm(flow.Testnet)
	simpleView := delta.NewDeltaView(nil)

	const computationLimit = 5

	type test struct {
		name           string
		code           string
		payerIsServAcc bool
		ok             bool
		expCompUsed    uint64
	}

	tests := []test{
		{
			name: "infinite while loop",
			code: `
		      while true {}
		    `,
			payerIsServAcc: false,
			ok:             false,
			expCompUsed:    computationLimit + 1,
		},
		{
			name: "limited while loop",
			code: `
              var i = 0
              while i < 5 {
                  i = i + 1
              }
            `,
			payerIsServAcc: false,
			ok:             false,
			expCompUsed:    computationLimit + 1,
		},
		{
			name: "too many for-in loop iterations",
			code: `
              for i in [1, 2, 3, 4, 5, 6, 7, 8, 9, 10] {}
            `,
			payerIsServAcc: false,
			ok:             false,
			expCompUsed:    computationLimit + 1,
		},
		{
			name: "too many for-in loop iterations",
			code: `
              for i in [1, 2, 3, 4, 5, 6, 7, 8, 9, 10] {}
            `,
			payerIsServAcc: true,
			ok:             true,
			expCompUsed:    11,
		},
		{
			name: "some for-in loop iterations",
			code: `
              for i in [1, 2, 3, 4] {}
            `,
			payerIsServAcc: false,
			ok:             true,
			expCompUsed:    5,
		},
	}

	for _, test := range tests {

		t.Run(test.name, func(t *testing.T) {
			ctx := fvm.NewContext(
				fvm.WithChain(chain),
				fvm.WithAuthorizationChecksEnabled(false),
				fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
			)

			script := []byte(
				fmt.Sprintf(
					`
                      transaction {
                          prepare() {
                              %s
                          }
                      }
                    `,
					test.code,
				),
			)

			txBody := flow.NewTransactionBody().
				SetScript(script).
				SetGasLimit(computationLimit)

			if test.payerIsServAcc {
				txBody.SetPayer(chain.ServiceAddress()).
					SetGasLimit(0)
			}
			tx := fvm.Transaction(txBody, 0)

			err := vm.Run(ctx, tx, simpleView)
			require.NoError(t, err)
			require.Equal(t, test.expCompUsed, tx.ComputationUsed)
			if test.ok {
				require.NoError(t, tx.Err)
			} else {
				require.Error(t, tx.Err)
			}

		})
	}
}

func TestStorageCapacity(t *testing.T) {
	t.Run("Storage capacity updates on FLOW transfer", newVMTest().
		withContextOptions(
			fvm.WithAuthorizationChecksEnabled(false),
			fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
			fvm.WithCadenceLogging(true),
		).
		withBootstrapProcedureOptions(
			fvm.WithStorageMBPerFLOW(10_0000_0000),
			fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		).
		run(func(
			t *testing.T,
			vm fvm.VM,
			chain flow.Chain,
			ctx fvm.Context,
			view state.View,
		) {
			service := chain.ServiceAddress()
			signer := createAccount(t, vm, chain, ctx, view)
			target := createAccount(t, vm, chain, ctx, view)

			// Transfer FLOW from service account to test accounts

			transferTxBody := transferTokensTx(chain).
				AddAuthorizer(service).
				AddArgument(jsoncdc.MustEncode(cadence.UFix64(1_000_000))).
				AddArgument(jsoncdc.MustEncode(cadence.NewAddress(signer))).
				SetProposalKey(service, 0, 0).
				SetPayer(service)
			tx := fvm.Transaction(transferTxBody, 0)
			err := vm.Run(ctx, tx, view)
			require.NoError(t, err)
			require.NoError(t, tx.Err)

			transferTxBody = transferTokensTx(chain).
				AddAuthorizer(service).
				AddArgument(jsoncdc.MustEncode(cadence.UFix64(1_000_000))).
				AddArgument(jsoncdc.MustEncode(cadence.NewAddress(target))).
				SetProposalKey(service, 0, 0).
				SetPayer(service)
			tx = fvm.Transaction(transferTxBody, 0)
			err = vm.Run(ctx, tx, view)
			require.NoError(t, err)
			require.NoError(t, tx.Err)

			// Perform test

			txBody := flow.NewTransactionBody().
				SetScript([]byte(fmt.Sprintf(`
					import FungibleToken from 0x%s
					import FlowToken from 0x%s
		
					transaction(target: Address) {
						prepare(signer: AuthAccount) {
							let receiverRef = getAccount(target)
								.getCapability(/public/flowTokenReceiver)
								.borrow<&{FungibleToken.Receiver}>()
								?? panic("Could not borrow receiver reference to the recipient''s Vault")
							
							let vaultRef = signer
								.borrow<&{FungibleToken.Provider}>(from: /storage/flowTokenVault)
								?? panic("Could not borrow reference to the owner''s Vault!")
							
							var cap0: UInt64 = signer.storageCapacity
							
							receiverRef.deposit(from: <- vaultRef.withdraw(amount: 0.0000001))
							
							var cap1: UInt64 = signer.storageCapacity
							
							log(cap0 - cap1)
						}
					}`,
					fvm.FungibleTokenAddress(chain),
					fvm.FlowTokenAddress(chain),
				))).
				AddArgument(jsoncdc.MustEncode(cadence.NewAddress(target))).
				AddAuthorizer(signer)

			tx = fvm.Transaction(txBody, 0)

			err = vm.Run(ctx, tx, view)
			require.NoError(t, err)
			require.NoError(t, tx.Err)

			require.Len(t, tx.Logs, 1)
			assert.Equal(t, tx.Logs[0], "1")
		}),
	)
}

func TestScriptContractMutationsFailure(t *testing.T) {
	t.Parallel()

	t.Run("contract additions are not committed",
		newVMTest().run(
			func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, view state.View) {

				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
				accounts, err := testutil.CreateAccounts(vm, view, privateKeys, chain)
				require.NoError(t, err)
				account := accounts[0]
				address := cadence.NewAddress(account)

				scriptCtx := fvm.NewContextFromParent(ctx)

				contract := "pub contract Foo {}"

				script := fvm.Script([]byte(fmt.Sprintf(`
				pub fun main(account: Address) {
					let acc = getAuthAccount(account)
					acc.contracts.add(name: "Foo", code: "%s".decodeHex())
				}`, hex.EncodeToString([]byte(contract))),
				)).WithArguments(
					jsoncdc.MustEncode(address),
				)

				err = vm.Run(scriptCtx, script, view)
				require.NoError(t, err)
				require.Error(t, script.Err)
				require.True(t, errors.IsCadenceRuntimeError(script.Err))
				// modifications to contracts are not supported in scripts
				require.True(t, errors.IsOperationNotSupportedError(script.Err))
			},
		),
	)

	t.Run("contract removals are not committed",
		newVMTest().run(
			func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, view state.View) {

				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				privateKey := privateKeys[0]
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
				accounts, err := testutil.CreateAccounts(vm, view, privateKeys, chain)
				require.NoError(t, err)
				account := accounts[0]
				address := cadence.NewAddress(account)

				subCtx := fvm.NewContextFromParent(ctx)

				contract := "pub contract Foo {}"

				txBody := flow.NewTransactionBody().SetScript([]byte(fmt.Sprintf(`
					transaction {
						prepare(signer: AuthAccount, service: AuthAccount) {
							signer.contracts.add(name: "Foo", code: "%s".decodeHex())
						}
					}
				`, hex.EncodeToString([]byte(contract))))).
					AddAuthorizer(account).
					AddAuthorizer(chain.ServiceAddress()).
					SetPayer(chain.ServiceAddress()).
					SetProposalKey(chain.ServiceAddress(), 0, 0)

				_ = testutil.SignPayload(txBody, account, privateKey)
				_ = testutil.SignEnvelope(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
				tx := fvm.Transaction(txBody, 0)
				err = vm.Run(subCtx, tx, view)
				require.NoError(t, err)
				require.NoError(t, tx.Err)

				script := fvm.Script([]byte(`
				pub fun main(account: Address) {
					let acc = getAuthAccount(account)
					let n = acc.contracts.names[0]
					acc.contracts.remove(name: n)
				}`,
				)).WithArguments(
					jsoncdc.MustEncode(address),
				)

				err = vm.Run(subCtx, script, view)
				require.NoError(t, err)
				require.Error(t, script.Err)
				require.True(t, errors.IsCadenceRuntimeError(script.Err))
				// modifications to contracts are not supported in scripts
				require.True(t, errors.IsOperationNotSupportedError(script.Err))
			},
		),
	)

	t.Run("contract updates are not committed",
		newVMTest().run(
			func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, view state.View) {

				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				privateKey := privateKeys[0]
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
				accounts, err := testutil.CreateAccounts(vm, view, privateKeys, chain)
				require.NoError(t, err)
				account := accounts[0]
				address := cadence.NewAddress(account)

				subCtx := fvm.NewContextFromParent(ctx)

				contract := "pub contract Foo {}"

				txBody := flow.NewTransactionBody().SetScript([]byte(fmt.Sprintf(`
					transaction {
						prepare(signer: AuthAccount, service: AuthAccount) {
							signer.contracts.add(name: "Foo", code: "%s".decodeHex())
						}
					}
				`, hex.EncodeToString([]byte(contract))))).
					AddAuthorizer(account).
					AddAuthorizer(chain.ServiceAddress()).
					SetPayer(chain.ServiceAddress()).
					SetProposalKey(chain.ServiceAddress(), 0, 0)

				_ = testutil.SignPayload(txBody, account, privateKey)
				_ = testutil.SignEnvelope(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
				tx := fvm.Transaction(txBody, 0)
				err = vm.Run(subCtx, tx, view)
				require.NoError(t, err)
				require.NoError(t, tx.Err)

				script := fvm.Script([]byte(fmt.Sprintf(`
				pub fun main(account: Address) {
					let acc = getAuthAccount(account)
					let n = acc.contracts.names[0]
					acc.contracts.update__experimental(name: n, code: "%s".decodeHex())
				}`, hex.EncodeToString([]byte(contract))))).WithArguments(
					jsoncdc.MustEncode(address),
				)

				err = vm.Run(subCtx, script, view)
				require.NoError(t, err)
				require.Error(t, script.Err)
				require.True(t, errors.IsCadenceRuntimeError(script.Err))
				// modifications to contracts are not supported in scripts
				require.True(t, errors.IsOperationNotSupportedError(script.Err))
			},
		),
	)
}

func TestScriptAccountKeyMutationsFailure(t *testing.T) {
	t.Parallel()

	t.Run("Account key additions are not committed",
		newVMTest().run(
			func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, view state.View) {

				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
				accounts, err := testutil.CreateAccounts(vm, view, privateKeys, chain)
				require.NoError(t, err)
				account := accounts[0]
				address := cadence.NewAddress(account)

				scriptCtx := fvm.NewContextFromParent(ctx)

				seed := make([]byte, crypto.KeyGenSeedMinLen)
				_, _ = rand.Read(seed)

				privateKey, _ := crypto.GeneratePrivateKey(crypto.ECDSAP256, seed)

				script := fvm.Script([]byte(`
					pub fun main(account: Address, k: [UInt8]) {
						let acc = getAuthAccount(account)
						acc.addPublicKey(k)
					}`,
				)).WithArguments(
					jsoncdc.MustEncode(address),
					jsoncdc.MustEncode(testutil.BytesToCadenceArray(
						privateKey.PublicKey().Encode(),
					)),
				)

				err = vm.Run(scriptCtx, script, view)
				require.NoError(t, err)
				require.Error(t, script.Err)
				require.True(t, errors.IsCadenceRuntimeError(script.Err))
				// modifications to public keys are not supported in scripts
				require.True(t, errors.IsOperationNotSupportedError(script.Err))
			},
		),
	)

	t.Run("Account key removals are not committed",
		newVMTest().run(
			func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, view state.View) {

				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
				accounts, err := testutil.CreateAccounts(vm, view, privateKeys, chain)
				require.NoError(t, err)
				account := accounts[0]
				address := cadence.NewAddress(account)

				scriptCtx := fvm.NewContextFromParent(ctx)

				script := fvm.Script([]byte(`
				pub fun main(account: Address) {
					let acc = getAuthAccount(account)
					acc.removePublicKey(0)
				}`,
				)).WithArguments(
					jsoncdc.MustEncode(address),
				)

				err = vm.Run(scriptCtx, script, view)
				require.NoError(t, err)
				require.Error(t, script.Err)
				require.True(t, errors.IsCadenceRuntimeError(script.Err))
				// modifications to public keys are not supported in scripts
				require.True(t, errors.IsOperationNotSupportedError(script.Err))
			},
		),
	)
}

func TestInteractionLimit(t *testing.T) {
	type testCase struct {
		name             string
		interactionLimit uint64
		require          func(t *testing.T, tx *fvm.TransactionProcedure)
	}

	testCases := []testCase{
		{
			name:             "high limit succeeds",
			interactionLimit: math.MaxUint64,
			require: func(t *testing.T, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Len(t, tx.Events, 5)
			},
		},
		{
			name:             "default limit succeeds",
			interactionLimit: fvm.DefaultMaxInteractionSize,
			require: func(t *testing.T, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Len(t, tx.Events, 5)
				unittest.EnsureEventsIndexSeq(t, tx.Events, flow.Testnet.Chain().ChainID())
			},
		},
		{
			name:             "low limit succeeds",
			interactionLimit: 170000,
			require: func(t *testing.T, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Len(t, tx.Events, 5)
				unittest.EnsureEventsIndexSeq(t, tx.Events, flow.Testnet.Chain().ChainID())
			},
		},
		{
			name:             "even lower low limit fails, and has only 3 events",
			interactionLimit: 5000,
			require: func(t *testing.T, tx *fvm.TransactionProcedure) {
				require.Error(t, tx.Err)
				require.Len(t, tx.Events, 3)
				unittest.EnsureEventsIndexSeq(t, tx.Events, flow.Testnet.Chain().ChainID())
			},
		},
	}

	// === setup ===
	// setup an address with some funds
	var privateKey flow.AccountPrivateKey
	var address flow.Address
	vmt, err := newVMTest().withBootstrapProcedureOptions(
		fvm.WithTransactionFee(fvm.DefaultTransactionFees),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithExecutionMemoryLimit(math.MaxUint64),
	).withContextOptions(
		fvm.WithTransactionFeesEnabled(true),
		fvm.WithAccountStorageLimit(true),
	).bootstrapWith(
		func(vm fvm.VM, chain flow.Chain, ctx fvm.Context, view state.View) error {
			// ==== Create an account ====
			var txBody *flow.TransactionBody
			privateKey, txBody = testutil.CreateAccountCreationTransaction(t, chain)

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			if err != nil {
				return err
			}

			tx := fvm.Transaction(txBody, 0)

			err = vm.Run(ctx, tx, view)
			if err != nil {
				return err
			}
			if tx.Err != nil {
				return tx.Err
			}

			accountCreatedEvents := filterAccountCreatedEvents(tx.Events)

			// read the address of the account created (e.g. "0x01" and convert it to flow.address)
			data, err := jsoncdc.Decode(nil, accountCreatedEvents[0].Payload)
			if err != nil {
				return err
			}
			address = flow.ConvertAddress(
				data.(cadence.Event).Fields[0].(cadence.Address))

			// ==== Transfer tokens to new account ====
			txBody = transferTokensTx(chain).
				AddAuthorizer(chain.ServiceAddress()).
				AddArgument(jsoncdc.MustEncode(cadence.UFix64(1_000_000))).
				AddArgument(jsoncdc.MustEncode(cadence.NewAddress(address)))

			txBody.SetProposalKey(chain.ServiceAddress(), 0, 1)
			txBody.SetPayer(chain.ServiceAddress())

			err = testutil.SignEnvelope(
				txBody,
				chain.ServiceAddress(),
				unittest.ServiceAccountPrivateKey,
			)
			if err != nil {
				return err
			}

			tx = fvm.Transaction(txBody, 0)

			err = vm.Run(ctx, tx, view)
			if err != nil {
				return err
			}
			if tx.Err != nil {
				return tx.Err
			}
			return nil
		},
	)
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.name, vmt.run(
			func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, view state.View) {
				// ==== Transfer funds with lowe interaction limit ====
				txBody := transferTokensTx(chain).
					AddAuthorizer(address).
					AddArgument(jsoncdc.MustEncode(cadence.UFix64(1))).
					AddArgument(jsoncdc.MustEncode(cadence.NewAddress(chain.ServiceAddress())))

				txBody.SetProposalKey(address, 0, 0)
				txBody.SetPayer(address)

				hasher, err := exeUtils.NewHasher(privateKey.HashAlgo)
				require.NoError(t, err)

				sig, err := txBody.Sign(txBody.EnvelopeMessage(), privateKey.PrivateKey, hasher)
				require.NoError(t, err)
				txBody.AddEnvelopeSignature(address, 0, sig)

				tx := fvm.Transaction(txBody, 0)

				// ==== IMPORTANT LINE ====
				ctx.MaxStateInteractionSize = tc.interactionLimit

				err = vm.Run(ctx, tx, view)
				require.NoError(t, err)
				tc.require(t, tx)
			}),
		)
	}
}

func TestAuthAccountCapabilities(t *testing.T) {

	t.Parallel()

	t.Run("transaction", func(t *testing.T) {

		t.Parallel()

		test := func(t *testing.T, allowAccountLinking bool) {
			newVMTest().
				withBootstrapProcedureOptions().
				withContextOptions(
					fvm.WithReusableCadenceRuntimePool(
						reusableRuntime.NewReusableCadenceRuntimePool(
							1,
							runtime.Config{
								AccountLinkingEnabled: true,
							},
						),
					),
				).
				run(
					func(
						t *testing.T,
						vm fvm.VM,
						chain flow.Chain,
						ctx fvm.Context,
						view state.View,
					) {
						// Create an account private key.
						privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
						privateKey := privateKeys[0]
						require.NoError(t, err)

						// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
						accounts, err := testutil.CreateAccounts(vm, view, privateKeys, chain)
						require.NoError(t, err)
						account := accounts[0]

						var pragma string
						if allowAccountLinking {
							pragma = "#allowAccountLinking"
						}

						code := fmt.Sprintf(
							`
							%s
							transaction {
								prepare(acct: AuthAccount) {
									acct.linkAccount(/private/foo)
								}
							}
							`,
							pragma,
						)

						txBody := flow.NewTransactionBody().
							SetScript([]byte(code)).
							AddAuthorizer(account).
							SetPayer(chain.ServiceAddress()).
							SetProposalKey(chain.ServiceAddress(), 0, 0)

						_ = testutil.SignPayload(txBody, account, privateKey)
						_ = testutil.SignEnvelope(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
						tx := fvm.Transaction(txBody, 0)

						err = vm.Run(ctx, tx, view)
						require.NoError(t, err)
						if allowAccountLinking {
							require.NoError(t, tx.Err)
						} else {
							require.Error(t, tx.Err)
						}
					},
				)(t)
		}

		t.Run("account linking allowed", func(t *testing.T) {
			test(t, true)
		})

		t.Run("account linking disallowed", func(t *testing.T) {
			test(t, false)
		})
	})

	t.Run("contract", func(t *testing.T) {

		t.Parallel()

		test := func(t *testing.T, allowAccountLinking bool) {
			newVMTest().
				withBootstrapProcedureOptions().
				withContextOptions(
					fvm.WithReusableCadenceRuntimePool(
						reusableRuntime.NewReusableCadenceRuntimePool(
							1,
							runtime.Config{
								AccountLinkingEnabled: true,
							},
						),
					),
					fvm.WithContractDeploymentRestricted(false),
				).
				run(
					func(
						t *testing.T,
						vm fvm.VM,
						chain flow.Chain,
						ctx fvm.Context,
						view state.View,
					) {
						// Create two private keys
						privateKeys, err := testutil.GenerateAccountPrivateKeys(2)
						require.NoError(t, err)

						// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
						accounts, err := testutil.CreateAccounts(vm, view, privateKeys, chain)
						require.NoError(t, err)

						// Deploy contract
						contractCode := `
							pub contract AccountLinker {
								pub fun link(_ account: AuthAccount) {
									account.linkAccount(/private/acct)
								}
							}
						`

						deployingContractScriptTemplate := `
							transaction {
								prepare(signer: AuthAccount) {
									signer.contracts.add(
										name: "AccountLinker",
										code: "%s".decodeHex()
									)
								}
							}
						`

						txBody := flow.NewTransactionBody().
							SetScript([]byte(fmt.Sprintf(
								deployingContractScriptTemplate,
								hex.EncodeToString([]byte(contractCode)),
							))).
							SetPayer(chain.ServiceAddress()).
							SetProposalKey(chain.ServiceAddress(), 0, 0).
							AddAuthorizer(accounts[0])
						_ = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
						_ = testutil.SignEnvelope(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)

						tx := fvm.Transaction(txBody, 0)
						err = vm.Run(ctx, tx, view)
						require.NoError(t, err)
						require.NoError(t, tx.Err)

						// Use contract

						var pragma string
						if allowAccountLinking {
							pragma = "#allowAccountLinking"
						}

						code := fmt.Sprintf(
							`
							%s
							import AccountLinker from %s
							transaction {
								prepare(acct: AuthAccount) {
									AccountLinker.link(acct)
								}
							}
							`,
							pragma,
							accounts[0].HexWithPrefix(),
						)

						txBody = flow.NewTransactionBody().
							SetScript([]byte(code)).
							AddAuthorizer(accounts[1]).
							SetPayer(chain.ServiceAddress()).
							SetProposalKey(chain.ServiceAddress(), 0, 1)

						_ = testutil.SignPayload(txBody, accounts[1], privateKeys[1])
						_ = testutil.SignEnvelope(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
						tx = fvm.Transaction(txBody, 1)

						err = vm.Run(ctx, tx, view)
						require.NoError(t, err)
						if allowAccountLinking {
							require.NoError(t, tx.Err)
						} else {
							require.Error(t, tx.Err)
						}
					},
				)(t)
		}

		t.Run("account linking allowed", func(t *testing.T) {
			test(t, true)
		})

		t.Run("account linking disallowed", func(t *testing.T) {
			test(t, false)
		})
	})
}

func TestAttachments(t *testing.T) {
	test := func(t *testing.T, attachmentsEnabled bool) {
		newVMTest().
			withBootstrapProcedureOptions().
			withContextOptions(
				fvm.WithReusableCadenceRuntimePool(
					reusableRuntime.NewReusableCadenceRuntimePool(
						1,
						runtime.Config{
							AttachmentsEnabled: attachmentsEnabled,
						},
					),
				),
			).
			run(
				func(
					t *testing.T,
					vm fvm.VM,
					chain flow.Chain,
					ctx fvm.Context,
					view state.View,
				) {

					script := fvm.Script([]byte(`

						pub resource R {}

						pub attachment A for R {}

						pub fun main() {
							let r <- create R()
							r[A]
							destroy r
						}
					`))

					err := vm.Run(ctx, script, view)
					require.NoError(t, err)

					if attachmentsEnabled {
						require.NoError(t, script.Err)
					} else {
						require.Error(t, script.Err)
						require.ErrorContains(t, script.Err, "attachments are not enabled")
					}
				},
			)(t)
	}

	t.Run("attachments enabled", func(t *testing.T) {
		test(t, true)
	})

	t.Run("attachments disabled", func(t *testing.T) {
		test(t, false)
	})
}
