package fvm_test

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	mockery "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	cadenceErrors "github.com/onflow/cadence/errors"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/sema"
	cadenceStdlib "github.com/onflow/cadence/stdlib"
	"github.com/onflow/cadence/test_utils/runtime_utils"
	"github.com/onflow/crypto"
	"github.com/onflow/flow-core-contracts/lib/go/contracts"
	bridge "github.com/onflow/flow-evm-bridge"
	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/test"

	"github.com/onflow/flow-go/engine/execution/testutil"
	exeUtils "github.com/onflow/flow-go/engine/execution/utils"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/blueprints"
	fvmCrypto "github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/environment"
	envMock "github.com/onflow/flow-go/fvm/environment/mock"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/evm/events"
	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/fvm/meter"
	reusableRuntime "github.com/onflow/flow-go/fvm/runtime"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/storage/snapshot/mock"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/storage/testutils"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

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
	f func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree),
) func(t *testing.T) {
	return func(t *testing.T) {
		baseOpts := []fvm.Option{
			// default chain is Testnet
			fvm.WithChain(flow.Testnet.Chain()),
			fvm.WithEntropyProvider(testutil.EntropyProviderFixture(nil)),
		}

		opts := append(baseOpts, vmt.contextOptions...)
		ctx := fvm.NewContext(opts...)

		chain := ctx.Chain
		vm := fvm.NewVirtualMachine()

		snapshotTree := snapshot.NewSnapshotTree(nil)

		baseBootstrapOpts := []fvm.BootstrapProcedureOption{
			fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
		}

		bootstrapOpts := append(baseBootstrapOpts, vmt.bootstrapOptions...)

		executionSnapshot, _, err := vm.Run(
			ctx,
			fvm.Bootstrap(unittest.ServiceAccountPublicKey, bootstrapOpts...),
			snapshotTree)
		require.NoError(t, err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

		f(t, vm, chain, ctx, snapshotTree)
	}
}

// bootstrapWith executes the bootstrap procedure and the custom bootstrap function
// and returns a prepared bootstrappedVmTest with all the state needed
func (vmt vmTest) bootstrapWith(
	bootstrap func(vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) (snapshot.SnapshotTree, error),
) (bootstrappedVmTest, error) {

	baseOpts := []fvm.Option{
		// default chain is Testnet
		fvm.WithChain(flow.Testnet.Chain()),
	}

	opts := append(baseOpts, vmt.contextOptions...)
	ctx := fvm.NewContext(opts...)

	chain := ctx.Chain
	vm := fvm.NewVirtualMachine()

	snapshotTree := snapshot.NewSnapshotTree(nil)

	baseBootstrapOpts := []fvm.BootstrapProcedureOption{
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
	}

	bootstrapOpts := append(baseBootstrapOpts, vmt.bootstrapOptions...)

	executionSnapshot, _, err := vm.Run(
		ctx,
		fvm.Bootstrap(unittest.ServiceAccountPublicKey, bootstrapOpts...),
		snapshotTree)
	if err != nil {
		return bootstrappedVmTest{}, err
	}

	snapshotTree = snapshotTree.Append(executionSnapshot)

	snapshotTree, err = bootstrap(vm, chain, ctx, snapshotTree)
	if err != nil {
		return bootstrappedVmTest{}, err
	}

	return bootstrappedVmTest{chain, ctx, snapshotTree}, nil
}

type bootstrappedVmTest struct {
	chain        flow.Chain
	ctx          fvm.Context
	snapshotTree snapshot.SnapshotTree
}

// run Runs a test from the bootstrapped state, without changing the bootstrapped state
func (vmt bootstrappedVmTest) run(
	f func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree),
) func(t *testing.T) {
	return func(t *testing.T) {
		f(t, fvm.NewVirtualMachine(), vmt.chain, vmt.ctx, vmt.snapshotTree)
	}
}

func TestHashing(t *testing.T) {

	t.Parallel()

	chain, vm := createChainAndVm(flow.Mainnet)

	ctx := fvm.NewContext(
		fvm.WithChain(chain),
		fvm.WithCadenceLogging(true),
	)

	snapshotTree := testutil.RootBootstrappedLedger(vm, ctx)

	hashScript := func(hashName string) []byte {
		return []byte(fmt.Sprintf(
			`
				import Crypto

				access(all)
				fun main(data: [UInt8]): [UInt8] {
					return Crypto.hash(data, algorithm: HashAlgorithm.%s)
				}
			`, hashName))
	}
	hashWithTagScript := func(hashName string) []byte {
		return []byte(fmt.Sprintf(
			`
				import Crypto

				access(all) fun main(data: [UInt8], tag: String): [UInt8] {
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

			_, output, err := vm.Run(ctx, script, snapshotTree)
			require.NoError(t, err)

			byteResult := make([]byte, 0)
			if err == nil && output.Err == nil {
				cadenceArray := output.Value.(cadence.Array)
				for _, value := range cadenceArray.Values {
					byteResult = append(byteResult, uint8(value.(cadence.UInt8)))
				}
			}

			c.Check(t, hex.EncodeToString(byteResult), output.Err, err)
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
			_, output, err := vm.Run(ctx, script, snapshotTree)
			require.NoError(t, err)
			require.NoError(t, output.Err)

			result1 := make([]byte, 0)
			cadenceArray := output.Value.(cadence.Array)
			for _, value := range cadenceArray.Values {
				result1 = append(result1, uint8(value.(cadence.UInt8)))
			}

			code = hashScript(algo.Name())
			script = fvm.Script(code)
			script = script.WithArguments(
				cadenceData,
			)
			_, output, err = vm.Run(ctx, script, snapshotTree)
			require.NoError(t, err)
			require.NoError(t, output.Err)

			result2 := make([]byte, 0)
			cadenceArray = output.Value.(cadence.Array)
			for _, value := range cadenceArray.Values {
				result2 = append(result2, uint8(value.(cadence.UInt8)))
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

	snapshotTree := snapshot.NewSnapshotTree(nil)

	txBody := flow.NewTransactionBody().
		SetScript([]byte(`transaction { prepare(signer: auth(BorrowValue) &Account) { Account(payer: signer) } }`)).
		AddAuthorizer(chain.ServiceAddress())

	t.Run("With service account enabled", func(t *testing.T) {
		executionSnapshot, output, err := vm.Run(
			ctxA,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)

		// transaction should fail on non-bootstrapped ledger
		require.Error(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)
	})

	t.Run("With service account disabled", func(t *testing.T) {
		ctxB := fvm.NewContextFromParent(
			ctxA,
			fvm.WithServiceAccount(false))

		_, output, err := vm.Run(
			ctxB,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)

		// transaction should succeed on non-bootstrapped ledger
		require.NoError(t, output.Err)
	})
}

func TestEventLimits(t *testing.T) {
	chain, vm := createChainAndVm(flow.Mainnet)

	ctx := fvm.NewContext(
		fvm.WithChain(chain),
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
	)

	snapshotTree := testutil.RootBootstrappedLedger(vm, ctx)

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
			prepare(signer: auth(AddContract) &Account) {
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

	executionSnapshot, output, err := vm.Run(
		ctx,
		fvm.Transaction(txBody, 0),
		snapshotTree)
	require.NoError(t, err)
	require.NoError(t, output.Err)

	snapshotTree = snapshotTree.Append(executionSnapshot)

	txBody = flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(`
		import TestContract from 0x%s
			transaction {
			prepare(acct: &Account) {}
			execute {
				TestContract.EmitEvent()
			}
		}`, chain.ServiceAddress()))).
		AddAuthorizer(chain.ServiceAddress())

	t.Run("With limits", func(t *testing.T) {
		txBody.Payer = unittest.RandomAddressFixture()

		executionSnapshot, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)

		// transaction should fail due to event size limit
		require.Error(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)
	})

	t.Run("With service account as payer", func(t *testing.T) {
		txBody.Payer = chain.ServiceAddress()

		_, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)

		unittest.EnsureEventsIndexSeq(t, output.Events, chain.ChainID())

		// transaction should not fail due to event size limit
		require.NoError(t, output.Err)
	})
}

// TestHappyPathSigning checks that a signing a transaction with `Sign` doesn't produce an error.
// Transaction verification tests are in `TestVerifySignatureFromTransaction`.
func TestHappyPathTransactionSigning(t *testing.T) {

	newVMTest().run(
		func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
			// Create an account private key.
			privateKey, err := testutil.GenerateAccountPrivateKey()
			require.NoError(t, err)

			// Bootstrap a ledger, creating accounts with the provided private
			// keys and the root account.
			snapshotTree, accounts, err := testutil.CreateAccounts(
				vm,
				snapshotTree,
				[]flow.AccountPrivateKey{privateKey},
				chain)
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

			_, output, err := vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)
			require.NoError(t, err)
			require.NoError(t, output.Err)
		},
	)
}

func TestTransactionFeeDeduction(t *testing.T) {
	getBalance := func(vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree, address flow.Address) uint64 {
		sc := systemcontracts.SystemContractsForChain(chain.ChainID())
		code := []byte(fmt.Sprintf(
			`
					import FungibleToken from 0x%s
					import FlowToken from 0x%s

					access(all) fun main(account: Address): UFix64 {
						let acct = getAccount(account)
						let vaultRef = acct.capabilities.borrow<&FlowToken.Vault>(/public/flowTokenBalance)
							?? panic("Could not borrow Balance reference to the Vault")

						return vaultRef.balance
					}
				`,
			sc.FungibleToken.Address.Hex(),
			sc.FlowToken.Address.Hex(),
		))
		script := fvm.Script(code).WithArguments(
			jsoncdc.MustEncode(cadence.NewAddress(address)),
		)

		_, output, err := vm.Run(ctx, script, snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)
		return uint64(output.Value.(cadence.UFix64))
	}

	type testCase struct {
		name          string
		fundWith      uint64
		tryToTransfer uint64
		gasLimit      uint64
		checkResult   func(t *testing.T, balanceBefore uint64, balanceAfter uint64, output fvm.ProcedureOutput)
	}

	txFees := uint64(1_000)              // 0.00001
	fundingAmount := uint64(100_000_000) // 1.0
	transferAmount := uint64(123_456)
	minimumStorageReservation := uint64(fvm.DefaultMinimumStorageReservation)

	chain := flow.Testnet.Chain()
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	depositedEvent := fmt.Sprintf("A.%s.FlowToken.TokensDeposited", sc.FlowToken.Address)
	withdrawnEvent := fmt.Sprintf("A.%s.FlowToken.TokensWithdrawn", sc.FlowToken.Address)
	feesDeductedEvent := fmt.Sprintf("A.%s.FlowFees.FeesDeducted", sc.FlowFees.Address)

	testCases := []testCase{
		{
			name:          "Transaction fees are deducted",
			fundWith:      fundingAmount,
			tryToTransfer: 0,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, output fvm.ProcedureOutput) {
				require.NoError(t, output.Err)
				require.Equal(t, txFees, balanceBefore-balanceAfter)
			},
		},
		{
			name:          "Transaction fee deduction emits events",
			fundWith:      fundingAmount,
			tryToTransfer: 0,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, output fvm.ProcedureOutput) {
				require.NoError(t, output.Err)

				var deposits []flow.Event
				var withdraws []flow.Event

				chain := flow.Testnet.Chain()
				for _, e := range output.Events {
					if string(e.Type) == depositedEvent {
						deposits = append(deposits, e)
					}
					if string(e.Type) == withdrawnEvent {
						withdraws = append(withdraws, e)
					}
				}

				unittest.EnsureEventsIndexSeq(t, output.Events, chain.ChainID())
				require.Len(t, deposits, 2)
				require.Len(t, withdraws, 2)
			},
		},
		{
			name:          "Transaction fees are deducted and tx is applied",
			fundWith:      fundingAmount,
			tryToTransfer: transferAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, output fvm.ProcedureOutput) {
				require.NoError(t, output.Err)
				require.Equal(t, txFees+transferAmount, balanceBefore-balanceAfter)
			},
		},
		{
			name:          "Transaction fees are deducted and fee deduction is emitted",
			fundWith:      fundingAmount,
			tryToTransfer: transferAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, output fvm.ProcedureOutput) {
				require.NoError(t, output.Err)
				chain := flow.Testnet.Chain()

				var feeDeduction flow.Event // fee deduction event
				for _, e := range output.Events {
					if string(e.Type) == feesDeductedEvent {
						feeDeduction = e
						break
					}
				}
				unittest.EnsureEventsIndexSeq(t, output.Events, chain.ChainID())
				require.NotEmpty(t, feeDeduction.Payload)

				payload, err := ccf.Decode(nil, feeDeduction.Payload)
				require.NoError(t, err)

				event := payload.(cadence.Event)

				fields := cadence.FieldsMappedByName(event)

				actualTXFees := fields["amount"]
				actualExecutionEffort := fields["executionEffort"]
				actualInclusionEffort := fields["inclusionEffort"]

				require.Equal(t,
					txFees,
					uint64(actualTXFees.(cadence.UFix64)),
				)
				// Inclusion effort should be equivalent to 1.0 UFix64
				require.Equal(t,
					uint64(100_000_000),
					uint64(actualInclusionEffort.(cadence.UFix64)),
				)
				// Execution effort should be non-0
				require.Greater(t, actualExecutionEffort, uint64(0))

			},
		},
		{
			name:          "If just enough balance, fees are deducted",
			fundWith:      txFees + transferAmount,
			tryToTransfer: transferAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, output fvm.ProcedureOutput) {
				require.NoError(t, output.Err)
				require.Equal(t, uint64(0), balanceAfter)
			},
		},
		{
			// this is an edge case that is not applicable to any network.
			// If storage limits were on this would fail due to storage limits
			name:          "If not enough balance, transaction succeeds and fees are deducted to 0",
			fundWith:      txFees,
			tryToTransfer: 1,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, output fvm.ProcedureOutput) {
				require.NoError(t, output.Err)
				require.Equal(t, uint64(0), balanceAfter)
			},
		},
		{
			name:          "If tx fails, fees are deducted",
			fundWith:      fundingAmount,
			tryToTransfer: 2 * fundingAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, output fvm.ProcedureOutput) {
				require.Error(t, output.Err)
				require.Equal(t, fundingAmount-txFees, balanceAfter)
			},
		},
		{
			name:          "If tx fails, fee deduction events are emitted",
			fundWith:      fundingAmount,
			tryToTransfer: 2 * fundingAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, output fvm.ProcedureOutput) {
				require.Error(t, output.Err)

				var deposits []flow.Event
				var withdraws []flow.Event

				chain := flow.Testnet.Chain()

				for _, e := range output.Events {
					if string(e.Type) == depositedEvent {
						deposits = append(deposits, e)
					}
					if string(e.Type) == withdrawnEvent {
						withdraws = append(withdraws, e)
					}
				}

				unittest.EnsureEventsIndexSeq(t, output.Events, chain.ChainID())
				require.Len(t, deposits, 1)
				require.Len(t, withdraws, 1)
			},
		},
		{
			name:          "If tx fails because of gas limit reached, fee deduction events are emitted",
			fundWith:      txFees + transferAmount,
			tryToTransfer: transferAmount,
			gasLimit:      uint64(2),
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, output fvm.ProcedureOutput) {
				require.ErrorContains(t, output.Err, "computation exceeds limit (2)")

				var deposits []flow.Event
				var withdraws []flow.Event

				chain := flow.Testnet.Chain()

				for _, e := range output.Events {
					if string(e.Type) == depositedEvent {
						deposits = append(deposits, e)
					}
					if string(e.Type) == withdrawnEvent {
						withdraws = append(withdraws, e)
					}
				}

				unittest.EnsureEventsIndexSeq(t, output.Events, chain.ChainID())
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
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, output fvm.ProcedureOutput) {
				require.NoError(t, output.Err)
				require.Equal(t, txFees, balanceBefore-balanceAfter)
			},
		},
		{
			name:          "Transaction fee deduction emits events",
			fundWith:      fundingAmount,
			tryToTransfer: 0,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, output fvm.ProcedureOutput) {
				require.NoError(t, output.Err)

				var deposits []flow.Event
				var withdraws []flow.Event

				chain := flow.Testnet.Chain()

				for _, e := range output.Events {
					if string(e.Type) == depositedEvent {
						deposits = append(deposits, e)
					}
					if string(e.Type) == withdrawnEvent {
						withdraws = append(withdraws, e)
					}
				}

				unittest.EnsureEventsIndexSeq(t, output.Events, chain.ChainID())
				require.Len(t, deposits, 2)
				require.Len(t, withdraws, 2)
			},
		},
		{
			name:          "Transaction fees are deducted and tx is applied",
			fundWith:      fundingAmount,
			tryToTransfer: transferAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, output fvm.ProcedureOutput) {
				require.NoError(t, output.Err)
				require.Equal(t, txFees+transferAmount, balanceBefore-balanceAfter)
			},
		},
		{
			name:          "If just enough balance, fees are deducted",
			fundWith:      txFees + transferAmount,
			tryToTransfer: transferAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, output fvm.ProcedureOutput) {
				require.NoError(t, output.Err)
				require.Equal(t, minimumStorageReservation, balanceAfter)
			},
		},
		{
			name:          "If tx fails, fees are deducted",
			fundWith:      fundingAmount,
			tryToTransfer: 2 * fundingAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, output fvm.ProcedureOutput) {
				require.Error(t, output.Err)
				require.Equal(t, fundingAmount-txFees+minimumStorageReservation, balanceAfter)
			},
		},
		{
			name:          "If tx fails, fee deduction events are emitted",
			fundWith:      fundingAmount,
			tryToTransfer: 2 * fundingAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, output fvm.ProcedureOutput) {
				require.Error(t, output.Err)

				var deposits []flow.Event
				var withdraws []flow.Event

				chain := flow.Testnet.Chain()

				for _, e := range output.Events {
					if string(e.Type) == depositedEvent {
						deposits = append(deposits, e)
					}
					if string(e.Type) == withdrawnEvent {
						withdraws = append(withdraws, e)
					}
				}

				unittest.EnsureEventsIndexSeq(t, output.Events, chain.ChainID())
				require.Len(t, deposits, 1)
				require.Len(t, withdraws, 1)
			},
		},
		{
			name:          "If balance at minimum, transaction fails, fees are deducted and fee deduction events are emitted",
			fundWith:      0,
			tryToTransfer: 0,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, output fvm.ProcedureOutput) {
				require.Error(t, output.Err)
				require.Equal(t, minimumStorageReservation-txFees, balanceAfter)

				var deposits []flow.Event
				var withdraws []flow.Event

				chain := flow.Testnet.Chain()

				for _, e := range output.Events {
					if string(e.Type) == depositedEvent {
						deposits = append(deposits, e)
					}
					if string(e.Type) == withdrawnEvent {
						withdraws = append(withdraws, e)
					}
				}

				unittest.EnsureEventsIndexSeq(t, output.Events, chain.ChainID())
				require.Len(t, deposits, 1)
				require.Len(t, withdraws, 1)
			},
		},
	}

	runTx := func(tc testCase) func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
		return func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
			// ==== Create an account ====
			privateKey, txBody := testutil.CreateAccountCreationTransaction(t, chain)

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			executionSnapshot, output, err := vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)
			require.NoError(t, err)
			require.NoError(t, output.Err)

			snapshotTree = snapshotTree.Append(executionSnapshot)

			require.Len(t, output.Events, 20)
			unittest.EnsureEventsIndexSeq(t, output.Events, chain.ChainID())

			accountCreatedEvents := filterAccountCreatedEvents(output.Events)

			require.Len(t, accountCreatedEvents, 1)

			// read the address of the account created (e.g. "0x01" and convert it to flow.address)
			data, err := ccf.Decode(nil, accountCreatedEvents[0].Payload)
			require.NoError(t, err)

			address := flow.ConvertAddress(
				cadence.SearchFieldByName(
					data.(cadence.Event),
					cadenceStdlib.AccountEventAddressParameter.Identifier,
				).(cadence.Address),
			)

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

			executionSnapshot, output, err = vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)
			require.NoError(t, err)
			require.NoError(t, output.Err)

			snapshotTree = snapshotTree.Append(executionSnapshot)

			balanceBefore := getBalance(vm, chain, ctx, snapshotTree, address)

			// ==== Transfer tokens from new account ====

			txBody = transferTokensTx(chain).
				AddAuthorizer(address).
				AddArgument(jsoncdc.MustEncode(cadence.UFix64(tc.tryToTransfer))).
				AddArgument(jsoncdc.MustEncode(cadence.NewAddress(chain.ServiceAddress())))

			txBody.SetProposalKey(address, 0, 0)
			txBody.SetPayer(address)

			if tc.gasLimit == 0 {
				txBody.SetComputeLimit(fvm.DefaultComputationLimit)
			} else {
				txBody.SetComputeLimit(tc.gasLimit)
			}

			err = testutil.SignEnvelope(
				txBody,
				address,
				privateKey,
			)
			require.NoError(t, err)

			executionSnapshot, output, err = vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)
			require.NoError(t, err)

			snapshotTree = snapshotTree.Append(executionSnapshot)

			balanceAfter := getBalance(vm, chain, ctx, snapshotTree, address)

			tc.checkResult(
				t,
				balanceBefore,
				balanceAfter,
				output,
			)
		}
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Transaction Fees %d: %s", i, tc.name), newVMTest().withBootstrapProcedureOptions(
			fvm.WithTransactionFee(fvm.DefaultTransactionFees),
			fvm.WithExecutionMemoryLimit(math.MaxUint64),
			fvm.WithExecutionEffortWeights(environment.MainnetExecutionEffortWeights),
			fvm.WithExecutionMemoryWeights(meter.DefaultMemoryWeights),
		).withContextOptions(
			fvm.WithTransactionFeesEnabled(true),
			fvm.WithChain(chain),
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
			fvm.WithExecutionEffortWeights(environment.MainnetExecutionEffortWeights),
			fvm.WithExecutionMemoryWeights(meter.DefaultMemoryWeights),
		).withContextOptions(
			fvm.WithTransactionFeesEnabled(true),
			fvm.WithAccountStorageLimit(true),
			fvm.WithChain(chain),
		).run(
			runTx(tc)),
		)
	}
}

func TestSettingExecutionWeights(t *testing.T) {

	// change the chain so that the metering settings are read from the service account
	chain := flow.Emulator.Chain()

	t.Run("transaction should fail with high weights", newVMTest().withBootstrapProcedureOptions(

		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
		fvm.WithExecutionEffortWeights(
			meter.ExecutionEffortWeights{
				common.ComputationKindLoop: 100_000 << meter.MeterExecutionInternalPrecisionBytes,
			},
		),
	).withContextOptions(
		fvm.WithChain(chain),
	).run(
		func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {

			txBody := flow.NewTransactionBody().
				SetScript([]byte(`
				transaction {
                  prepare(signer: &Account) {
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

			_, output, err := vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)
			require.NoError(t, err)

			require.True(t, errors.IsComputationLimitExceededError(output.Err))
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
		fvm.WithChain(chain),
	).run(
		func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
			// Create an account private key.
			privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
			require.NoError(t, err)

			// Bootstrap a ledger, creating accounts with the provided private
			// keys and the root account.
			snapshotTree, accounts, err := testutil.CreateAccounts(
				vm,
				snapshotTree,
				privateKeys,
				chain)
			require.NoError(t, err)

			txBody := flow.NewTransactionBody().
				SetScript([]byte(`
				transaction {
                  prepare(signer: &Account) {
					var a = 1
                  }
                }
			`)).
				SetProposalKey(accounts[0], 0, 0).
				AddAuthorizer(accounts[0]).
				SetPayer(accounts[0])

			err = testutil.SignTransaction(txBody, accounts[0], privateKeys[0], 0)
			require.NoError(t, err)

			_, output, err := vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)
			require.NoError(t, err)
			require.Greater(t, output.MemoryEstimate, uint64(highWeight))

			require.True(t, errors.IsMemoryLimitExceededError(output.Err))
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
		fvm.WithChain(chain),
	).run(
		func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {

			txBody := flow.NewTransactionBody().
				SetScript([]byte(`
				transaction {
                  prepare(signer: &Account) {
					var a = 1
                  }
                }
			`)).
				SetProposalKey(chain.ServiceAddress(), 0, 0).
				AddAuthorizer(chain.ServiceAddress()).
				SetPayer(chain.ServiceAddress())

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			_, output, err := vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)
			require.NoError(t, err)
			require.Greater(t, output.MemoryEstimate, uint64(highWeight))

			require.NoError(t, output.Err)
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
	).withContextOptions(
		fvm.WithChain(chain),
	).run(
		func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
			privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
			require.NoError(t, err)

			snapshotTree, accounts, err := testutil.CreateAccounts(
				vm,
				snapshotTree,
				privateKeys,
				chain)
			require.NoError(t, err)

			// This transaction is specially designed to use a lot of breaks
			// as the weight for breaks is much higher than usual.
			// putting a `while true {break}` in a loop does not use the same amount of memory.
			txBody := flow.NewTransactionBody().
				SetScript([]byte(`
				transaction {
					prepare(signer: &Account) {
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

			_, output, err := vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)
			require.NoError(t, err)
			// There are 100 breaks and each break uses 1_000_000 memory
			require.Greater(t, output.MemoryEstimate, uint64(100_000_000))

			require.True(t, errors.IsMemoryLimitExceededError(output.Err))
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
	).withContextOptions(
		fvm.WithChain(chain),
	).run(
		func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
			txBody := flow.NewTransactionBody().
				SetScript([]byte(`
				transaction {
                  prepare(signer: auth(BorrowValue) &Account) {
					Account(payer: signer)
                  }
                }
			`)).
				SetProposalKey(chain.ServiceAddress(), 0, 0).
				AddAuthorizer(chain.ServiceAddress()).
				SetPayer(chain.ServiceAddress())

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			_, output, err := vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)
			require.NoError(t, err)

			require.True(t, errors.IsComputationLimitExceededError(output.Err))
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
	).withContextOptions(
		fvm.WithChain(chain),
	).run(
		func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {

			txBody := flow.NewTransactionBody().
				SetScript([]byte(`
				transaction {
                  prepare(signer: auth(BorrowValue) &Account) {
					Account(payer: signer)
                  }
                }
			`)).
				SetProposalKey(chain.ServiceAddress(), 0, 0).
				AddAuthorizer(chain.ServiceAddress()).
				SetPayer(chain.ServiceAddress())

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			_, output, err := vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)
			require.NoError(t, err)

			require.True(t, errors.IsComputationLimitExceededError(output.Err))
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
	).withContextOptions(
		fvm.WithChain(chain),
	).run(
		func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
			txBody := flow.NewTransactionBody().
				SetScript([]byte(`
				transaction {
                  prepare(signer: auth(BorrowValue) &Account) {
					Account(payer: signer)
                  }
                }
			`)).
				SetProposalKey(chain.ServiceAddress(), 0, 0).
				AddAuthorizer(chain.ServiceAddress()).
				SetPayer(chain.ServiceAddress())

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			_, output, err := vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)
			require.NoError(t, err)

			require.True(t, errors.IsComputationLimitExceededError(output.Err))
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
		fvm.WithChain(chain),
	).run(
		func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
			// Use the maximum amount of computation so that the transaction still passes.
			loops := uint64(996)
			executionEffortNeededToCheckStorage := uint64(1)
			maxExecutionEffort := uint64(997)
			txBody := flow.NewTransactionBody().
				SetScript([]byte(fmt.Sprintf(`
				transaction() {prepare(signer: &Account){var i=0;  while i < %d {i = i +1 } } execute{}}
			`, loops))).
				SetProposalKey(chain.ServiceAddress(), 0, 0).
				AddAuthorizer(chain.ServiceAddress()).
				SetPayer(chain.ServiceAddress()).
				SetComputeLimit(maxExecutionEffort)

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			executionSnapshot, output, err := vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)
			require.NoError(t, err)
			require.NoError(t, output.Err)

			snapshotTree = snapshotTree.Append(executionSnapshot)

			// expected computation used is number of loops + 1 (from the storage limit check).
			require.Equal(t, loops+executionEffortNeededToCheckStorage, output.ComputationUsed)

			// increasing the number of loops should fail the transaction.
			loops = loops + 1
			txBody = flow.NewTransactionBody().
				SetScript([]byte(fmt.Sprintf(`
				transaction() {prepare(signer: &Account){var i=0;  while i < %d {i = i +1 } } execute{}}
			`, loops))).
				SetProposalKey(chain.ServiceAddress(), 0, 1).
				AddAuthorizer(chain.ServiceAddress()).
				SetPayer(chain.ServiceAddress()).
				SetComputeLimit(maxExecutionEffort)

			err = testutil.SignTransactionAsServiceAccount(txBody, 1, chain)
			require.NoError(t, err)

			_, output, err = vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)
			require.NoError(t, err)

			require.ErrorContains(t, output.Err, "computation exceeds limit (997)")
			// expected computation used is still number of loops + 1 (from the storage limit check).
			require.Equal(t, loops+executionEffortNeededToCheckStorage, output.ComputationUsed)

			for _, event := range output.Events {
				// the fee deduction event should only contain the max gas worth of execution effort.
				if strings.Contains(string(event.Type), "FlowFees.FeesDeducted") {
					v, err := ccf.Decode(nil, event.Payload)
					require.NoError(t, err)

					ev := v.(cadence.Event)

					actualExecutionEffort := cadence.SearchFieldByName(ev, "executionEffort")

					require.Equal(
						t,
						maxExecutionEffort,
						uint64(actualExecutionEffort.(cadence.UFix64)),
					)
				}
			}
			unittest.EnsureEventsIndexSeq(t, output.Events, chain.ChainID())
		},
	))

	t.Run("transaction with more accounts touched uses more computation", newVMTest().withBootstrapProcedureOptions(
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
		fvm.WithTransactionFee(fvm.DefaultTransactionFees),
		fvm.WithExecutionEffortWeights(
			meter.ExecutionEffortWeights{
				common.ComputationKindStatement: 0,
				// only count loops
				// the storage check has a loop
				common.ComputationKindLoop:               1 << meter.MeterExecutionInternalPrecisionBytes,
				common.ComputationKindFunctionInvocation: 0,
			},
		),
	).withContextOptions(
		fvm.WithAccountStorageLimit(true),
		fvm.WithTransactionFeesEnabled(true),
		fvm.WithMemoryLimit(math.MaxUint64),
		fvm.WithChain(chain),
	).run(
		func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
			// Create an account private key.
			privateKeys, err := testutil.GenerateAccountPrivateKeys(5)
			require.NoError(t, err)

			// Bootstrap a ledger, creating accounts with the provided
			// private keys and the root account.
			snapshotTree, accounts, err := testutil.CreateAccounts(
				vm,
				snapshotTree,
				privateKeys,
				chain)
			require.NoError(t, err)

			sc := systemcontracts.SystemContractsForChain(chain.ChainID())

			// create a transaction without loops so only the looping in the storage check is counted.
			txBody := flow.NewTransactionBody().
				SetScript([]byte(fmt.Sprintf(`
					import FungibleToken from 0x%s
					import FlowToken from 0x%s

					transaction() {
						let sentVault: @{FungibleToken.Vault}

						prepare(signer: auth(BorrowValue) &Account) {
							let vaultRef = signer.storage.borrow<auth(FungibleToken.Withdraw) &FlowToken.Vault>(from: /storage/flowTokenVault)
								?? panic("Could not borrow reference to the owner's Vault!")

							self.sentVault <- vaultRef.withdraw(amount: 5.0)
						}

						execute {
							let recipient1 = getAccount(%s)
							let recipient2 = getAccount(%s)
							let recipient3 = getAccount(%s)
							let recipient4 = getAccount(%s)
							let recipient5 = getAccount(%s)

							let receiverRef1 = recipient1.capabilities.borrow<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)
								?? panic("Could not borrow receiver reference to the recipient's Vault")
							let receiverRef2 = recipient2.capabilities.borrow<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)
								?? panic("Could not borrow receiver reference to the recipient's Vault")
							let receiverRef3 = recipient3.capabilities.borrow<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)
								?? panic("Could not borrow receiver reference to the recipient's Vault")
							let receiverRef4 = recipient4.capabilities.borrow<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)
								?? panic("Could not borrow receiver reference to the recipient's Vault")
							let receiverRef5 = recipient5.capabilities.borrow<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)
								?? panic("Could not borrow receiver reference to the recipient's Vault")

							receiverRef1.deposit(from: <-self.sentVault.withdraw(amount: 1.0))
							receiverRef2.deposit(from: <-self.sentVault.withdraw(amount: 1.0))
							receiverRef3.deposit(from: <-self.sentVault.withdraw(amount: 1.0))
							receiverRef4.deposit(from: <-self.sentVault.withdraw(amount: 1.0))
							receiverRef5.deposit(from: <-self.sentVault.withdraw(amount: 1.0))

							destroy self.sentVault
						}
					}`,
					sc.FungibleToken.Address,
					sc.FlowToken.Address,
					accounts[0].HexWithPrefix(),
					accounts[1].HexWithPrefix(),
					accounts[2].HexWithPrefix(),
					accounts[3].HexWithPrefix(),
					accounts[4].HexWithPrefix(),
				))).
				SetProposalKey(chain.ServiceAddress(), 0, 0).
				AddAuthorizer(chain.ServiceAddress()).
				SetPayer(chain.ServiceAddress())

			err = testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			_, output, err := vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)
			require.NoError(t, err)
			require.NoError(t, output.Err)

			// The storage check should loop once for each of the five accounts created +
			// once for the service account
			require.Equal(t, uint64(5+1), output.ComputationUsed)
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
        access(all) fun main(): UInt64 {

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
                storageUsed = account.storage.used
            }

            return storageUsed
        }
	`)

	address, err := hex.DecodeString("2a3c4c2581cef731")
	require.NoError(t, err)

	accountStatusId := flow.AccountStatusRegisterID(
		flow.BytesToAddress(address))

	status := environment.NewAccountStatus()
	status.SetStorageUsed(5)

	_, output, err := vm.Run(
		ctx,
		fvm.Script(code),
		snapshot.MapStorageSnapshot{
			accountStatusId: status.ToBytes(),
		})
	require.NoError(t, err)

	require.Equal(t, cadence.NewUInt64(5), output.Value)
}

func TestEnforcingComputationLimit(t *testing.T) {
	t.Parallel()

	chain, vm := createChainAndVm(flow.Testnet)

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
				SetComputeLimit(computationLimit)

			if test.payerIsServAcc {
				txBody.SetPayer(chain.ServiceAddress()).
					SetComputeLimit(0)
			}
			tx := fvm.Transaction(txBody, 0)

			_, output, err := vm.Run(ctx, tx, nil)
			require.NoError(t, err)
			require.Equal(t, test.expCompUsed, output.ComputationUsed)
			if test.ok {
				require.NoError(t, output.Err)
			} else {
				require.Error(t, output.Err)
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
			snapshotTree snapshot.SnapshotTree,
		) {
			service := chain.ServiceAddress()
			snapshotTree, signer := createAccount(
				t,
				vm,
				chain,
				ctx,
				snapshotTree)
			snapshotTree, target := createAccount(
				t,
				vm,
				chain,
				ctx,
				snapshotTree)

			// Transfer FLOW from service account to test accounts

			transferTxBody := transferTokensTx(chain).
				AddAuthorizer(service).
				AddArgument(jsoncdc.MustEncode(cadence.UFix64(1_000_000))).
				AddArgument(jsoncdc.MustEncode(cadence.NewAddress(signer))).
				SetProposalKey(service, 0, 0).
				SetPayer(service)

			executionSnapshot, output, err := vm.Run(
				ctx,
				fvm.Transaction(transferTxBody, 0),
				snapshotTree)
			require.NoError(t, err)
			require.NoError(t, output.Err)

			snapshotTree = snapshotTree.Append(executionSnapshot)

			transferTxBody = transferTokensTx(chain).
				AddAuthorizer(service).
				AddArgument(jsoncdc.MustEncode(cadence.UFix64(1_000_000))).
				AddArgument(jsoncdc.MustEncode(cadence.NewAddress(target))).
				SetProposalKey(service, 0, 0).
				SetPayer(service)

			executionSnapshot, output, err = vm.Run(
				ctx,
				fvm.Transaction(transferTxBody, 0),
				snapshotTree)
			require.NoError(t, err)
			require.NoError(t, output.Err)

			snapshotTree = snapshotTree.Append(executionSnapshot)

			// Perform test
			sc := systemcontracts.SystemContractsForChain(chain.ChainID())

			txBody := flow.NewTransactionBody().
				SetScript([]byte(fmt.Sprintf(
					`
					import FungibleToken from 0x%s
					import FlowToken from 0x%s

					transaction(target: Address) {
						prepare(signer: auth(BorrowValue) &Account) {
							let receiverRef = getAccount(target)
								.capabilities.borrow<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)
								?? panic("Could not borrow receiver reference to the recipient''s Vault")

							let vaultRef = signer.storage
								.borrow<auth(FungibleToken.Withdraw) &FlowToken.Vault>(from: /storage/flowTokenVault)
								?? panic("Could not borrow reference to the owner''s Vault!")

							var cap0: UInt64 = signer.storage.capacity

							receiverRef.deposit(from: <- vaultRef.withdraw(amount: 0.0000001))

							var cap1: UInt64 = signer.storage.capacity

							log(cap0 - cap1)
						}
					}`,
					sc.FungibleToken.Address.Hex(),
					sc.FlowToken.Address.Hex(),
				))).
				AddArgument(jsoncdc.MustEncode(cadence.NewAddress(target))).
				AddAuthorizer(signer)

			_, output, err = vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)
			require.NoError(t, err)
			require.NoError(t, output.Err)

			require.Len(t, output.Logs, 1)
			require.Equal(t, output.Logs[0], "1")
		}),
	)
}

func TestScriptContractMutationsFailure(t *testing.T) {
	t.Parallel()

	t.Run("contract additions are not committed",
		newVMTest().run(
			func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided
				// private keys and the root account.
				snapshotTree, accounts, err := testutil.CreateAccounts(
					vm,
					snapshotTree,
					privateKeys,
					chain)
				require.NoError(t, err)
				account := accounts[0]
				address := cadence.NewAddress(account)

				scriptCtx := fvm.NewContextFromParent(ctx)

				contract := "access(all) contract Foo {}"

				script := fvm.Script([]byte(fmt.Sprintf(`
				access(all) fun main(account: Address) {
					let acc = getAuthAccount<auth(AddContract) &Account>(account)
					acc.contracts.add(name: "Foo", code: "%s".decodeHex())
				}`, hex.EncodeToString([]byte(contract))),
				)).WithArguments(
					jsoncdc.MustEncode(address),
				)

				_, output, err := vm.Run(scriptCtx, script, snapshotTree)
				require.NoError(t, err)
				require.Error(t, output.Err)
				require.True(t, errors.IsCadenceRuntimeError(output.Err))
				// modifications to contracts are not supported in scripts
				require.True(t, errors.IsOperationNotSupportedError(output.Err))
			},
		),
	)

	t.Run("contract removals are not committed",
		newVMTest().run(
			func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				privateKey := privateKeys[0]
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided
				// private keys and the root account.
				snapshotTree, accounts, err := testutil.CreateAccounts(
					vm,
					snapshotTree,
					privateKeys,
					chain)
				require.NoError(t, err)
				account := accounts[0]
				address := cadence.NewAddress(account)

				subCtx := fvm.NewContextFromParent(ctx)

				contract := "access(all) contract Foo {}"

				txBody := flow.NewTransactionBody().SetScript([]byte(fmt.Sprintf(`
					transaction {
						prepare(signer: auth(AddContract) &Account, service: &Account) {
							signer.contracts.add(name: "Foo", code: "%s".decodeHex())
						}
					}
				`, hex.EncodeToString([]byte(contract))))).
					AddAuthorizer(account).
					AddAuthorizer(chain.ServiceAddress()).
					SetPayer(chain.ServiceAddress()).
					SetProposalKey(chain.ServiceAddress(), 0, 0)

				_ = testutil.SignPayload(txBody, account, privateKey)
				_ = testutil.SignEnvelope(
					txBody,
					chain.ServiceAddress(),
					unittest.ServiceAccountPrivateKey)

				executionSnapshot, output, err := vm.Run(
					subCtx,
					fvm.Transaction(txBody, 0),
					snapshotTree)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				snapshotTree = snapshotTree.Append(executionSnapshot)

				script := fvm.Script([]byte(`
				access(all) fun main(account: Address) {
					let acc = getAuthAccount<auth(RemoveContract) &Account>(account)
					let n = acc.contracts.names[0]
					acc.contracts.remove(name: n)
				}`,
				)).WithArguments(
					jsoncdc.MustEncode(address),
				)

				_, output, err = vm.Run(subCtx, script, snapshotTree)
				require.NoError(t, err)
				require.Error(t, output.Err)
				require.True(t, errors.IsCadenceRuntimeError(output.Err))
				// modifications to contracts are not supported in scripts
				require.True(t, errors.IsOperationNotSupportedError(output.Err))
			},
		),
	)

	t.Run("contract updates are not committed",
		newVMTest().run(
			func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				privateKey := privateKeys[0]
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided
				// private keys and the root account.
				snapshotTree, accounts, err := testutil.CreateAccounts(
					vm,
					snapshotTree,
					privateKeys,
					chain)
				require.NoError(t, err)
				account := accounts[0]
				address := cadence.NewAddress(account)

				subCtx := fvm.NewContextFromParent(ctx)

				contract := "access(all) contract Foo {}"

				txBody := flow.NewTransactionBody().SetScript([]byte(fmt.Sprintf(`
					transaction {
						prepare(signer: auth(AddContract) &Account, service: &Account) {
							signer.contracts.add(name: "Foo", code: "%s".decodeHex())
						}
					}
				`, hex.EncodeToString([]byte(contract))))).
					AddAuthorizer(account).
					AddAuthorizer(chain.ServiceAddress()).
					SetPayer(chain.ServiceAddress()).
					SetProposalKey(chain.ServiceAddress(), 0, 0)

				_ = testutil.SignPayload(txBody, account, privateKey)
				_ = testutil.SignEnvelope(
					txBody,
					chain.ServiceAddress(),
					unittest.ServiceAccountPrivateKey)

				executionSnapshot, output, err := vm.Run(
					subCtx,
					fvm.Transaction(txBody, 0),
					snapshotTree)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				snapshotTree = snapshotTree.Append(executionSnapshot)

				script := fvm.Script([]byte(fmt.Sprintf(`
				access(all) fun main(account: Address) {
					let acc = getAuthAccount<auth(UpdateContract) &Account>(account)
					let n = acc.contracts.names[0]
					acc.contracts.update(name: n, code: "%s".decodeHex())
				}`, hex.EncodeToString([]byte(contract))))).WithArguments(
					jsoncdc.MustEncode(address),
				)

				_, output, err = vm.Run(subCtx, script, snapshotTree)
				require.NoError(t, err)
				require.Error(t, output.Err)
				require.True(t, errors.IsCadenceRuntimeError(output.Err))
				// modifications to contracts are not supported in scripts
				require.True(t, errors.IsOperationNotSupportedError(output.Err))
			},
		),
	)
}

func TestScriptAccountKeyMutationsFailure(t *testing.T) {
	t.Parallel()

	t.Run("Account key additions are not committed",
		newVMTest().run(
			func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided
				// private keys and the root account.
				snapshotTree, accounts, err := testutil.CreateAccounts(
					vm,
					snapshotTree,
					privateKeys,
					chain)
				require.NoError(t, err)
				account := accounts[0]
				address := cadence.NewAddress(account)

				scriptCtx := fvm.NewContextFromParent(ctx)

				seed := make([]byte, crypto.KeyGenSeedMinLen)
				_, _ = rand.Read(seed)

				privateKey, _ := crypto.GeneratePrivateKey(crypto.ECDSAP256, seed)

				script := fvm.Script([]byte(`
					access(all) fun main(account: Address, k: [UInt8]) {
						let acc = getAuthAccount<auth(AddKey) &Account>(account)
						acc.keys.add(
							publicKey: PublicKey(
                                publicKey: k,
                                signatureAlgorithm: SignatureAlgorithm.ECDSA_P256
                            ),
                            hashAlgorithm: HashAlgorithm.SHA3_256,
                            weight: 100.0
						)
					}`,
				)).WithArguments(
					jsoncdc.MustEncode(address),
					jsoncdc.MustEncode(testutil.BytesToCadenceArray(
						privateKey.PublicKey().Encode(),
					)),
				)

				_, output, err := vm.Run(scriptCtx, script, snapshotTree)
				require.NoError(t, err)
				require.Error(t, output.Err)
				require.True(t, errors.IsCadenceRuntimeError(output.Err))
				// modifications to public keys are not supported in scripts
				require.True(t, errors.IsOperationNotSupportedError(output.Err))
			},
		),
	)

	t.Run("Account key removals are not committed",
		newVMTest().run(
			func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided
				// private keys and the root account.
				snapshotTree, accounts, err := testutil.CreateAccounts(
					vm,
					snapshotTree,
					privateKeys,
					chain)
				require.NoError(t, err)
				account := accounts[0]
				address := cadence.NewAddress(account)

				scriptCtx := fvm.NewContextFromParent(ctx)

				script := fvm.Script([]byte(`
				access(all) fun main(account: Address) {
					let acc = getAuthAccount<auth(RevokeKey) &Account>(account)
					acc.keys.revoke(keyIndex: 0)
				}`,
				)).WithArguments(
					jsoncdc.MustEncode(address),
				)

				_, output, err := vm.Run(scriptCtx, script, snapshotTree)
				require.NoError(t, err)
				require.Error(t, output.Err)
				require.True(t, errors.IsCadenceRuntimeError(output.Err))
				// modifications to public keys are not supported in scripts
				require.True(t, errors.IsOperationNotSupportedError(output.Err))
			},
		),
	)
}

func TestScriptExecutionLimit(t *testing.T) {

	t.Parallel()

	chain := flow.Emulator.Chain()

	script := fvm.Script([]byte(`
		access(all) fun main() {
			var s: Int256 = 1024102410241024
			var i: Int256 = 0
			var a: Int256 = 7
			var b: Int256 = 5
			var c: Int256 = 2

			while i < 150000 {
				s = s * a
				s = s / b
				s = s / c
				i = i + 1
			}
		}
	`))

	bootstrapProcedureOptions := []fvm.BootstrapProcedureOption{
		fvm.WithTransactionFee(fvm.DefaultTransactionFees),
		fvm.WithExecutionMemoryLimit(math.MaxUint32),
		fvm.WithExecutionEffortWeights(map[common.ComputationKind]uint64{
			common.ComputationKindStatement:          1569,
			common.ComputationKindLoop:               1569,
			common.ComputationKindFunctionInvocation: 1569,
			environment.ComputationKindGetValue:      808,
			environment.ComputationKindCreateAccount: 2837670,
			environment.ComputationKindSetValue:      765,
		}),
		fvm.WithExecutionMemoryWeights(meter.DefaultMemoryWeights),
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
	}

	t.Run("Exceeding computation limit",
		newVMTest().withBootstrapProcedureOptions(
			bootstrapProcedureOptions...,
		).withContextOptions(
			fvm.WithTransactionFeesEnabled(true),
			fvm.WithAccountStorageLimit(true),
			fvm.WithComputationLimit(10000),
			fvm.WithChain(chain),
		).run(
			func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
				scriptCtx := fvm.NewContextFromParent(ctx)

				_, output, err := vm.Run(scriptCtx, script, snapshotTree)
				require.NoError(t, err)
				require.Error(t, output.Err)
				require.True(t, errors.IsComputationLimitExceededError(output.Err))
				require.ErrorContains(t, output.Err, "computation exceeds limit (10000)")
				require.GreaterOrEqual(t, output.ComputationUsed, uint64(10000))
				require.GreaterOrEqual(t, output.MemoryEstimate, uint64(548020260))
			},
		),
	)

	t.Run("Sufficient computation limit",
		newVMTest().withBootstrapProcedureOptions(
			bootstrapProcedureOptions...,
		).withContextOptions(
			fvm.WithTransactionFeesEnabled(true),
			fvm.WithAccountStorageLimit(true),
			fvm.WithComputationLimit(20000),
			fvm.WithChain(chain),
		).run(
			func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
				scriptCtx := fvm.NewContextFromParent(ctx)

				_, output, err := vm.Run(scriptCtx, script, snapshotTree)
				require.NoError(t, err)
				require.NoError(t, output.Err)
				require.GreaterOrEqual(t, output.ComputationUsed, uint64(17955))
				require.GreaterOrEqual(t, output.MemoryEstimate, uint64(984017413))
			},
		),
	)
}

func TestInteractionLimit(t *testing.T) {
	type testCase struct {
		name             string
		interactionLimit uint64
		require          func(t *testing.T, output fvm.ProcedureOutput)
	}

	testCases := []testCase{
		{
			name:             "high limit succeeds",
			interactionLimit: math.MaxUint64,
			require: func(t *testing.T, output fvm.ProcedureOutput) {
				require.NoError(t, output.Err)
				require.Len(t, output.Events, 9)
			},
		},
		{
			name:             "default limit succeeds",
			interactionLimit: fvm.DefaultMaxInteractionSize,
			require: func(t *testing.T, output fvm.ProcedureOutput) {
				require.NoError(t, output.Err)
				require.Len(t, output.Events, 9)
				unittest.EnsureEventsIndexSeq(t, output.Events, flow.Testnet.Chain().ChainID())
			},
		},
		{
			name:             "low limit succeeds",
			interactionLimit: 170000,
			require: func(t *testing.T, output fvm.ProcedureOutput) {
				require.NoError(t, output.Err)
				require.Len(t, output.Events, 9)
				unittest.EnsureEventsIndexSeq(t, output.Events, flow.Testnet.Chain().ChainID())
			},
		},
		{
			name:             "even lower low limit fails, and has only 5 events",
			interactionLimit: 5000,
			require: func(t *testing.T, output fvm.ProcedureOutput) {
				require.Error(t, output.Err)
				require.Len(t, output.Events, 5)
				unittest.EnsureEventsIndexSeq(t, output.Events, flow.Testnet.Chain().ChainID())
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
		func(vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) (snapshot.SnapshotTree, error) {
			// ==== Create an account ====
			var txBody *flow.TransactionBody
			privateKey, txBody = testutil.CreateAccountCreationTransaction(t, chain)

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			if err != nil {
				return snapshotTree, err
			}

			executionSnapshot, output, err := vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)
			if err != nil {
				return snapshotTree, err
			}

			snapshotTree = snapshotTree.Append(executionSnapshot)

			if output.Err != nil {
				return snapshotTree, output.Err
			}

			accountCreatedEvents := filterAccountCreatedEvents(output.Events)

			// read the address of the account created (e.g. "0x01" and convert it to flow.address)
			data, err := ccf.Decode(nil, accountCreatedEvents[0].Payload)
			if err != nil {
				return snapshotTree, err
			}

			address = flow.ConvertAddress(
				cadence.SearchFieldByName(
					data.(cadence.Event),
					cadenceStdlib.AccountEventAddressParameter.Identifier,
				).(cadence.Address),
			)

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
				return snapshotTree, err
			}

			executionSnapshot, output, err = vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)
			if err != nil {
				return snapshotTree, err
			}

			return snapshotTree.Append(executionSnapshot), output.Err
		},
	)
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.name, vmt.run(
			func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
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

				// ==== IMPORTANT LINE ====
				ctx.MaxStateInteractionSize = tc.interactionLimit

				_, output, err := vm.Run(
					ctx,
					fvm.Transaction(txBody, 0),
					snapshotTree)
				require.NoError(t, err)
				tc.require(t, output)
			}),
		)
	}
}

func TestAttachments(t *testing.T) {

	newVMTest().
		withBootstrapProcedureOptions().
		run(
			func(
				t *testing.T,
				vm fvm.VM,
				chain flow.Chain,
				ctx fvm.Context,
				snapshotTree snapshot.SnapshotTree,
			) {
				script := fvm.Script([]byte(`

						access(all) resource R {}

						access(all) attachment A for R {}

						access(all) fun main() {
							let r <- create R()
							r[A]
							destroy r
						}
					`))

				_, output, err := vm.Run(ctx, script, snapshotTree)
				require.NoError(t, err)
				require.NoError(t, output.Err)

			},
		)(t)

}

func TestCapabilityControllers(t *testing.T) {
	test := func(t *testing.T) {
		newVMTest().
			withBootstrapProcedureOptions().
			withContextOptions(
				fvm.WithReusableCadenceRuntimePool(
					reusableRuntime.NewReusableCadenceRuntimePool(
						1,
						runtime.Config{},
					),
				),
			).
			run(func(
				t *testing.T,
				vm fvm.VM,
				chain flow.Chain,
				ctx fvm.Context,
				snapshotTree snapshot.SnapshotTree,
			) {
				txBody := flow.NewTransactionBody().
					SetScript([]byte(`
						transaction {
						  prepare(signer: auth(Capabilities) &Account) {
							let cap = signer.capabilities.storage.issue<&Int>(/storage/foo)
							assert(cap.id == 6)

							let cap2 = signer.capabilities.storage.issue<&String>(/storage/bar)
							assert(cap2.id == 7)
						  }
						}
					`)).
					SetProposalKey(chain.ServiceAddress(), 0, 0).
					AddAuthorizer(chain.ServiceAddress()).
					SetPayer(chain.ServiceAddress())

				err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
				require.NoError(t, err)

				_, output, err := vm.Run(
					ctx,
					fvm.Transaction(txBody, 0),
					snapshotTree)
				require.NoError(t, err)
				require.NoError(t, output.Err)
			},
			)(t)
	}

	test(t)

}

func TestStorageIterationWithBrokenValues(t *testing.T) {

	t.Parallel()

	newVMTest().
		withBootstrapProcedureOptions().
		withContextOptions(
			fvm.WithReusableCadenceRuntimePool(
				reusableRuntime.NewReusableCadenceRuntimePool(
					1,
					runtime.Config{},
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
				snapshotTree snapshot.SnapshotTree,
			) {
				// Create a private key
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				require.NoError(t, err)

				// Bootstrap a ledger, creating an account with the provided private key and the root account.
				snapshotTree, accounts, err := testutil.CreateAccounts(
					vm,
					snapshotTree,
					privateKeys,
					chain,
				)
				require.NoError(t, err)

				contractA := `
				    access(all) contract A {
						access(all) struct interface Foo{}
					}
				`

				updatedContractA := `
				    access(all) contract A {
						access(all) struct interface Foo{
							access(all) fun hello()
						}
					}
				`

				contractB := fmt.Sprintf(`
				    import A from %s

				    access(all) contract B {
						access(all) struct Bar : A.Foo {}

						access(all) struct interface Foo2{}
					}`,
					accounts[0].HexWithPrefix(),
				)

				contractC := fmt.Sprintf(`
				    import B from %s
				    import A from %s

				    access(all) contract C {
						access(all) struct Bar : A.Foo, B.Foo2 {}

						access(all) struct interface Foo3{}
					}`,
					accounts[0].HexWithPrefix(),
					accounts[0].HexWithPrefix(),
				)

				contractD := fmt.Sprintf(`
				    import C from %s
				    import B from %s
				    import A from %s

				    access(all) contract D {
						access(all) struct Bar : A.Foo, B.Foo2, C.Foo3 {}
					}`,
					accounts[0].HexWithPrefix(),
					accounts[0].HexWithPrefix(),
					accounts[0].HexWithPrefix(),
				)

				var sequenceNumber uint64 = 0

				runTransaction := func(code []byte) {
					txBody := flow.NewTransactionBody().
						SetScript(code).
						SetPayer(chain.ServiceAddress()).
						SetProposalKey(chain.ServiceAddress(), 0, sequenceNumber).
						AddAuthorizer(accounts[0])

					_ = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
					_ = testutil.SignEnvelope(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)

					executionSnapshot, output, err := vm.Run(
						ctx,
						fvm.Transaction(txBody, 0),
						snapshotTree,
					)
					require.NoError(t, err)
					require.NoError(t, output.Err)

					snapshotTree = snapshotTree.Append(executionSnapshot)

					// increment sequence number
					sequenceNumber++
				}

				// Deploy `A`
				runTransaction(runtime_utils.DeploymentTransaction(
					"A",
					[]byte(contractA),
				))

				// Deploy `B`
				runTransaction(runtime_utils.DeploymentTransaction(
					"B",
					[]byte(contractB),
				))

				// Deploy `C`
				runTransaction(runtime_utils.DeploymentTransaction(
					"C",
					[]byte(contractC),
				))

				// Deploy `D`
				runTransaction(runtime_utils.DeploymentTransaction(
					"D",
					[]byte(contractD),
				))

				// Store values
				runTransaction([]byte(fmt.Sprintf(
					`
					import D from %s
					import C from %s
					import B from %s

					transaction {
						prepare(signer: auth(Capabilities, Storage) &Account) {
							signer.storage.save("Hello, World!", to: /storage/a)
							signer.storage.save(["one", "two", "three"], to: /storage/b)
							signer.storage.save(D.Bar(), to: /storage/c)
							signer.storage.save(C.Bar(), to: /storage/d)
							signer.storage.save(B.Bar(), to: /storage/e)

							let aCap = signer.capabilities.storage.issue<&String>(/storage/a)
							signer.capabilities.publish(aCap, at: /public/a)

							let bCap = signer.capabilities.storage.issue<&[String]>(/storage/b)
							signer.capabilities.publish(bCap, at: /public/b)

							let cCap = signer.capabilities.storage.issue<&D.Bar>(/storage/c)
							signer.capabilities.publish(cCap, at: /public/c)

							let dCap = signer.capabilities.storage.issue<&C.Bar>(/storage/d)
							signer.capabilities.publish(dCap, at: /public/d)

							let eCap = signer.capabilities.storage.issue<&B.Bar>(/storage/e)
							signer.capabilities.publish(eCap, at: /public/e)
						}
					}`,
					accounts[0].HexWithPrefix(),
					accounts[0].HexWithPrefix(),
					accounts[0].HexWithPrefix(),
				)))

				// Update `A`, such that `B`, `C` and `D` are now broken.
				runTransaction(runtime_utils.UpdateTransaction(
					"A",
					[]byte(updatedContractA),
				))

				// Iterate stored values
				runTransaction([]byte(
					`
					transaction {
						prepare(account: auth(Storage) &Account) {
							var total = 0
							account.storage.forEachPublic(fun (path: PublicPath, type: Type): Bool {
								let cap = account.capabilities.get<&AnyStruct>(path)
								if cap.check() {
									total = total + 1
								}
                                return true
							})
							assert(total == 2, message:"found ".concat(total.toString()))

							total = 0
							account.storage.forEachStored(fun (path: StoragePath, type: Type): Bool {
								if account.storage.check<AnyStruct>(from: path) {
								    account.storage.copy<AnyStruct>(from: path)
								    total = total + 1
								}
                                return true
							})

							assert(total == 2, message:"found ".concat(total.toString()))
						}
					}`,
				))
			},
		)(t)
}

func TestEntropyCallOnlyOkIfAllowed(t *testing.T) {
	source := testutil.EntropyProviderFixture(nil)

	test := func(t *testing.T, allowed bool) {
		newVMTest().
			withBootstrapProcedureOptions().
			withContextOptions(
				fvm.WithRandomSourceHistoryCallAllowed(allowed),
				fvm.WithEntropyProvider(source),
			).
			run(func(
				t *testing.T,
				vm fvm.VM,
				chain flow.Chain,
				ctx fvm.Context,
				snapshotTree snapshot.SnapshotTree,
			) {
				txBody := flow.NewTransactionBody().
					SetScript([]byte(`
						transaction {
						  prepare() {
							randomSourceHistory()
						  }
						}
					`)).
					SetProposalKey(chain.ServiceAddress(), 0, 0).
					SetPayer(chain.ServiceAddress())

				err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
				require.NoError(t, err)

				_, output, err := vm.Run(
					ctx,
					fvm.Transaction(txBody, 0),
					snapshotTree)
				require.NoError(t, err)

				if allowed {
					require.NoError(t, output.Err)
				} else {
					require.Error(t, output.Err)
					require.True(t, errors.HasErrorCode(output.Err, errors.ErrCodeOperationNotSupportedError))
				}
			},
			)(t)
	}

	t.Run("enabled", func(t *testing.T) {
		test(t, true)
	})

	t.Run("disabled", func(t *testing.T) {
		test(t, false)
	})
}

func TestEntropyCallExpectsNoParameters(t *testing.T) {
	source := testutil.EntropyProviderFixture(nil)
	newVMTest().
		withBootstrapProcedureOptions().
		withContextOptions(
			fvm.WithRandomSourceHistoryCallAllowed(true),
			fvm.WithEntropyProvider(source),
		).
		run(func(
			t *testing.T,
			vm fvm.VM,
			chain flow.Chain,
			ctx fvm.Context,
			snapshotTree snapshot.SnapshotTree,
		) {
			txBody := flow.NewTransactionBody().
				SetScript([]byte(`
						transaction {
						  prepare() {
							randomSourceHistory("foo")
						  }
						}
					`)).
				SetProposalKey(chain.ServiceAddress(), 0, 0).
				SetPayer(chain.ServiceAddress())

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			_, output, err := vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)
			require.NoError(t, err)

			require.ErrorContains(t, output.Err, "too many arguments")
		},
		)(t)
}

func TestTransientNetworkCoreContractAddresses(t *testing.T) {

	// This test ensures that the transient networks have the correct core contract addresses.
	newVMTest().
		run(
			func(
				t *testing.T,
				vm fvm.VM,
				chain flow.Chain,
				ctx fvm.Context,
				snapshotTree snapshot.SnapshotTree,
			) {
				sc := systemcontracts.SystemContractsForChain(chain.ChainID())

				for _, contract := range sc.All() {
					txnState := testutils.NewSimpleTransaction(snapshotTree)
					accounts := environment.NewAccounts(txnState)

					yes, err := accounts.ContractExists(contract.Name, contract.Address)
					require.NoError(t, err)
					require.True(t, yes, "contract %s does not exist", contract.Name)
				}
			})
}

func TestEVM(t *testing.T) {
	blocks := new(envMock.Blocks)
	block1 := unittest.BlockFixture()
	blocks.On("ByHeightFrom",
		block1.Header.Height,
		block1.Header,
	).Return(block1.Header, nil)

	ctxOpts := []fvm.Option{
		// default is testnet, but testnet has a special EVM storage contract location
		// so we have to use emulator here so that the EVM storage contract is deployed
		// to the 5th address
		fvm.WithChain(flow.Emulator.Chain()),
		fvm.WithEVMEnabled(true),
		fvm.WithBlocks(blocks),
		fvm.WithBlockHeader(block1.Header),
		fvm.WithCadenceLogging(true),
	}

	t.Run("successful transaction", newVMTest().
		withBootstrapProcedureOptions(fvm.WithSetupEVMEnabled(true)).
		withContextOptions(ctxOpts...).
		run(func(
			t *testing.T,
			vm fvm.VM,
			chain flow.Chain,
			ctx fvm.Context,
			snapshotTree snapshot.SnapshotTree,
		) {
			// generate test address
			genArr := make([]cadence.Value, 20)
			for i := range genArr {
				genArr[i] = cadence.UInt8(i)
			}
			addrBytes := cadence.NewArray(genArr).WithType(stdlib.EVMAddressBytesCadenceType)
			encodedArg, err := jsoncdc.Encode(addrBytes)
			require.NoError(t, err)

			sc := systemcontracts.SystemContractsForChain(chain.ChainID())

			txBody := flow.NewTransactionBody().
				SetScript([]byte(fmt.Sprintf(`
						import EVM from %s

						transaction(bytes: [UInt8; 20]) {
							execute {
								let addr = EVM.EVMAddress(bytes: bytes)
								log(addr)
							}
						}
					`, sc.EVMContract.Address.HexWithPrefix()))).
				SetProposalKey(chain.ServiceAddress(), 0, 0).
				SetPayer(chain.ServiceAddress()).
				AddArgument(encodedArg)

			err = testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			_, output, err := vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)

			require.NoError(t, err)
			require.NoError(t, output.Err)
			require.Len(t, output.Logs, 1)
			require.Equal(t, output.Logs[0], fmt.Sprintf(
				"A.%s.EVM.EVMAddress(bytes: %s)",
				sc.EVMContract.Address,
				addrBytes.String(),
			))
		}),
	)

	// this test makes sure the execution error is correctly handled and returned as a correct type
	t.Run("execution reverted", newVMTest().
		withBootstrapProcedureOptions(fvm.WithSetupEVMEnabled(true)).
		withContextOptions(ctxOpts...).
		run(func(
			t *testing.T,
			vm fvm.VM,
			chain flow.Chain,
			ctx fvm.Context,
			snapshotTree snapshot.SnapshotTree,
		) {
			sc := systemcontracts.SystemContractsForChain(chain.ChainID())
			script := fvm.Script([]byte(fmt.Sprintf(`
				import EVM from %s

				access(all) fun main() {
					let bal = EVM.Balance(attoflow: 1000000000000000000)
					let acc <- EVM.createCadenceOwnedAccount()

					// withdraw insufficient balance
					destroy acc.withdraw(balance: bal)
					destroy acc
				}
			`, sc.EVMContract.Address.HexWithPrefix())))

			_, output, err := vm.Run(ctx, script, snapshotTree)

			require.NoError(t, err)
			require.Error(t, output.Err)
			require.True(t, errors.IsEVMError(output.Err))

			// make sure error is not treated as internal error by Cadence
			var internal cadenceErrors.InternalError
			require.False(t, errors.As(output.Err, &internal))
		}),
	)

	// this test makes sure the EVM error is correctly returned as an error and has a correct type
	// we have implemented a snapshot wrapper to return an error from the EVM
	t.Run("internal evm error handling", newVMTest().
		withBootstrapProcedureOptions(fvm.WithSetupEVMEnabled(true)).
		withContextOptions(ctxOpts...).
		run(func(
			t *testing.T,
			vm fvm.VM,
			chain flow.Chain,
			ctx fvm.Context,
			snapshotTree snapshot.SnapshotTree,
		) {
			sc := systemcontracts.SystemContractsForChain(chain.ChainID())

			tests := []struct {
				err        error
				errChecker func(error) bool
			}{{
				types.ErrNotImplemented,
				types.IsAFatalError,
			}, {
				types.NewStateError(fmt.Errorf("test state error")),
				types.IsAStateError,
			}}

			for _, e := range tests {
				// this mock will return an error we provide with the test once it starts to access address allocator registers
				// that is done to make sure the error is coming out of EVM execution
				errStorage := &mock.StorageSnapshot{}
				errStorage.
					On("Get", mockery.AnythingOfType("flow.RegisterID")).
					Return(func(id flow.RegisterID) (flow.RegisterValue, error) {
						if id.Key == "LatestBlock" || id.Key == "LatestBlockProposal" {
							return nil, e.err
						}
						return snapshotTree.Get(id)
					})

				script := fvm.Script([]byte(fmt.Sprintf(`
					import EVM from %s

					access(all)
                    fun main() {
						destroy <- EVM.createCadenceOwnedAccount()
					}
				`, sc.EVMContract.Address.HexWithPrefix())))

				_, output, err := vm.Run(ctx, script, errStorage)

				require.NoError(t, output.Err)
				require.Error(t, err)
				// make sure error it's the right type of error
				require.True(t, e.errChecker(err), "error is not of the right type")
			}
		}),
	)

	t.Run("deploy contract code", newVMTest().
		withBootstrapProcedureOptions(fvm.WithSetupEVMEnabled(true)).
		withContextOptions(ctxOpts...).
		run(func(
			t *testing.T,
			vm fvm.VM,
			chain flow.Chain,
			ctx fvm.Context,
			snapshotTree snapshot.SnapshotTree,
		) {
			sc := systemcontracts.SystemContractsForChain(chain.ChainID())

			txBody := flow.NewTransactionBody().
				SetScript([]byte(fmt.Sprintf(`
					import FungibleToken from %s
					import FlowToken from %s
					import EVM from %s

					transaction() {
						prepare(acc: auth(Storage) &Account) {
							let vaultRef = acc.storage
                                .borrow<auth(FungibleToken.Withdraw) &FlowToken.Vault>(from: /storage/flowTokenVault)
							    ?? panic("Could not borrow reference to the owner's Vault!")

							let evmHeartbeat = acc.storage
								.borrow<&EVM.Heartbeat>(from: /storage/EVMHeartbeat)
								?? panic("Couldn't borrow EVM.Heartbeat Resource")

							let acc <- EVM.createCadenceOwnedAccount()
							let amount <- vaultRef.withdraw(amount: 0.0000001) as! @FlowToken.Vault
							acc.deposit(from: <- amount)
							destroy acc

							// commit blocks
							evmHeartbeat.heartbeat()
						}
					}`,
					sc.FungibleToken.Address.HexWithPrefix(),
					sc.FlowToken.Address.HexWithPrefix(),
					sc.FlowServiceAccount.Address.HexWithPrefix(), // TODO this should be sc.EVM.Address not found there???
				))).
				SetProposalKey(chain.ServiceAddress(), 0, 0).
				AddAuthorizer(chain.ServiceAddress()).
				SetPayer(chain.ServiceAddress())

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			ctx = fvm.NewContextFromParent(ctx, fvm.WithEVMEnabled(true))
			_, output, err := vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)

			require.NoError(t, err)
			require.NoError(t, output.Err)
			require.Len(t, output.Events, 6)

			txExe, blockExe := output.Events[3], output.Events[5]
			txExecutedID := common.NewAddressLocation(
				nil,
				common.Address(sc.EVMContract.Address),
				string(events.EventTypeTransactionExecuted),
			).ID()
			blockExecutedID := common.NewAddressLocation(
				nil,
				common.Address(sc.EVMContract.Address),
				string(events.EventTypeBlockExecuted),
			).ID()
			assert.Equal(t, txExecutedID, string(txExe.Type))
			assert.Equal(t, blockExecutedID, string(blockExe.Type))

			// convert events to type ids
			eventTypeIDs := make([]common.TypeID, 0, len(output.Events))

			for _, event := range output.Events {
				eventTypeIDs = append(eventTypeIDs, common.TypeID(event.Type))
			}

			assert.ElementsMatch(
				t,
				[]common.TypeID{
					common.TypeID(txExecutedID),
					"A.f8d6e0586b0a20c7.EVM.CadenceOwnedAccountCreated",
					"A.ee82856bf20e2aa6.FungibleToken.Withdrawn",
					common.TypeID(txExecutedID),
					"A.f8d6e0586b0a20c7.EVM.FLOWTokensDeposited",
					common.TypeID(blockExecutedID),
				},
				eventTypeIDs,
			)
		}),
	)
}

func TestVMBridge(t *testing.T) {
	blocks := new(envMock.Blocks)
	block1 := unittest.BlockFixture()
	blocks.On("ByHeightFrom",
		block1.Header.Height,
		block1.Header,
	).Return(block1.Header, nil)

	ctxOpts := []fvm.Option{
		// default is testnet, but testnet has a special EVM storage contract location
		// so we have to use emulator here so that the EVM storage contract is deployed
		// to the 5th address
		fvm.WithChain(flow.Emulator.Chain()),
		fvm.WithEVMEnabled(true),
		fvm.WithBlocks(blocks),
		fvm.WithBlockHeader(block1.Header),
		fvm.WithCadenceLogging(true),
		fvm.WithContractDeploymentRestricted(false),
	}

	t.Run("successful FT Type Onboarding and Bridging", newVMTest().
		withBootstrapProcedureOptions(fvm.WithSetupEVMEnabled(true), fvm.WithSetupVMBridgeEnabled(true)).
		withContextOptions(ctxOpts...).
		run(func(
			t *testing.T,
			vm fvm.VM,
			chain flow.Chain,
			ctx fvm.Context,
			snapshotTree snapshot.SnapshotTree,
		) {

			sc := systemcontracts.SystemContractsForChain(chain.ChainID())

			env := sc.AsTemplateEnv()

			bridgeEnv := bridge.Environment{
				CrossVMNFTAddress:                     env.ServiceAccountAddress,
				CrossVMTokenAddress:                   env.ServiceAccountAddress,
				FlowEVMBridgeHandlerInterfacesAddress: env.ServiceAccountAddress,
				IBridgePermissionsAddress:             env.ServiceAccountAddress,
				ICrossVMAddress:                       env.ServiceAccountAddress,
				ICrossVMAssetAddress:                  env.ServiceAccountAddress,
				IEVMBridgeNFTMinterAddress:            env.ServiceAccountAddress,
				IEVMBridgeTokenMinterAddress:          env.ServiceAccountAddress,
				IFlowEVMNFTBridgeAddress:              env.ServiceAccountAddress,
				IFlowEVMTokenBridgeAddress:            env.ServiceAccountAddress,
				FlowEVMBridgeAddress:                  env.ServiceAccountAddress,
				FlowEVMBridgeAccessorAddress:          env.ServiceAccountAddress,
				FlowEVMBridgeConfigAddress:            env.ServiceAccountAddress,
				FlowEVMBridgeHandlersAddress:          env.ServiceAccountAddress,
				FlowEVMBridgeNFTEscrowAddress:         env.ServiceAccountAddress,
				FlowEVMBridgeResolverAddress:          env.ServiceAccountAddress,
				FlowEVMBridgeTemplatesAddress:         env.ServiceAccountAddress,
				FlowEVMBridgeTokenEscrowAddress:       env.ServiceAccountAddress,
				FlowEVMBridgeUtilsAddress:             env.ServiceAccountAddress,
				ArrayUtilsAddress:                     env.ServiceAccountAddress,
				ScopedFTProvidersAddress:              env.ServiceAccountAddress,
				SerializeAddress:                      env.ServiceAccountAddress,
				SerializeMetadataAddress:              env.ServiceAccountAddress,
				StringUtilsAddress:                    env.ServiceAccountAddress,
			}

			// Create an account private key.
			privateKey, err := testutil.GenerateAccountPrivateKey()
			require.NoError(t, err)

			// Create accounts with the provided private
			// key and the root account.
			snapshotTree, accounts, err := testutil.CreateAccounts(
				vm,
				snapshotTree,
				[]flow.AccountPrivateKey{privateKey},
				chain)
			require.NoError(t, err)

			txBody := blueprints.TransferFlowTokenTransaction(env, chain.ServiceAddress(), accounts[0], "2.0")

			err = testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			executionSnapshot, output, err := vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)

			require.NoError(t, err)
			require.NoError(t, output.Err)

			snapshotTree = snapshotTree.Append(executionSnapshot)

			// Deploy the ExampleToken contract
			tokenContract := contracts.ExampleToken(env)
			tokenContractName := "ExampleToken"
			txBody = blueprints.DeployContractTransaction(
				accounts[0],
				tokenContract,
				tokenContractName,
			)

			err = testutil.SignTransaction(txBody, accounts[0], privateKey, 0)
			require.NoError(t, err)

			executionSnapshot, output, err = vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)

			require.NoError(t, err)
			require.NoError(t, output.Err)

			snapshotTree = snapshotTree.Append(executionSnapshot)

			// Onboard the Fungible Token Type
			typeToOnboard := "A." + accounts[0].String() + "." + tokenContractName + ".Vault"

			txBody = blueprints.OnboardToBridgeByTypeIDTransaction(env, bridgeEnv, accounts[0], typeToOnboard)

			err = testutil.SignTransaction(txBody, accounts[0], privateKey, 1)
			require.NoError(t, err)

			executionSnapshot, output, err = vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)

			snapshotTree = snapshotTree.Append(executionSnapshot)

			require.NoError(t, err)
			require.NoError(t, output.Err)
			require.Len(t, output.Events, 7)
			for _, event := range output.Events {
				if strings.Contains(string(event.Type), "Onboarded") {
					// decode the event payload
					data, _ := ccf.Decode(nil, event.Payload)
					// get the contractAddress field from the event
					typeOnboarded := cadence.SearchFieldByName(
						data.(cadence.Event),
						"type",
					).(cadence.String)

					require.Equal(t, typeToOnboard, typeOnboarded.String()[1:len(typeOnboarded)+1])
				}
			}

			// Create COA in the new account
			txBody = blueprints.CreateCOATransaction(env, bridgeEnv, accounts[0])
			err = testutil.SignTransaction(txBody, accounts[0], privateKey, 2)
			require.NoError(t, err)

			executionSnapshot, output, err = vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)

			require.NoError(t, err)
			require.NoError(t, output.Err)

			snapshotTree = snapshotTree.Append(executionSnapshot)

			// Bridge the Fungible Token to EVM
			txBody = blueprints.BridgeFTToEVMTransaction(env, bridgeEnv, accounts[0], typeToOnboard, "1.0")
			err = testutil.SignTransaction(txBody, accounts[0], privateKey, 3)
			require.NoError(t, err)

			executionSnapshot, output, err = vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)

			require.NoError(t, err)
			require.NoError(t, output.Err)

			snapshotTree = snapshotTree.Append(executionSnapshot)

			// Confirm that the FT is escrowed
			script := blueprints.GetEscrowedTokenBalanceScript(env, bridgeEnv)

			arguments := []cadence.Value{
				cadence.String(typeToOnboard),
			}

			encodedArguments := make([][]byte, 0, len(arguments))
			for _, argument := range arguments {
				encodedArguments = append(encodedArguments, jsoncdc.MustEncode(argument))
			}

			_, output, err = vm.Run(
				ctx,
				fvm.Script(script).
					WithArguments(encodedArguments...),
				snapshotTree)
			require.NoError(t, err)
			require.NoError(t, output.Err)

			result := output.Value.(cadence.Optional).Value
			expected, _ := cadence.NewUFix64("1.0")
			require.Equal(t, expected, result)

			// Bridge the tokens back to Cadence
			txBody = blueprints.BridgeFTFromEVMTransaction(env, bridgeEnv, accounts[0], typeToOnboard, 1000000000000000000)

			err = testutil.SignTransaction(txBody, accounts[0], privateKey, 4)
			require.NoError(t, err)

			executionSnapshot, output, err = vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)

			require.NoError(t, err)
			require.NoError(t, output.Err)

			snapshotTree = snapshotTree.Append(executionSnapshot)

			// Confirm that the FT is no longer escrowed
			script = blueprints.GetEscrowedTokenBalanceScript(env, bridgeEnv)

			arguments = []cadence.Value{
				cadence.String(typeToOnboard),
			}

			encodedArguments = make([][]byte, 0, len(arguments))
			for _, argument := range arguments {
				encodedArguments = append(encodedArguments, jsoncdc.MustEncode(argument))
			}

			_, output, err = vm.Run(
				ctx,
				fvm.Script(script).
					WithArguments(encodedArguments...),
				snapshotTree)
			require.NoError(t, err)
			require.NoError(t, output.Err)

			result = output.Value.(cadence.Optional).Value
			expected, _ = cadence.NewUFix64("0.0")
			require.Equal(t, expected, result)
		}),
	)

	t.Run("successful NFT Type Onboarding and Bridging", newVMTest().
		withBootstrapProcedureOptions(fvm.WithSetupEVMEnabled(true), fvm.WithSetupVMBridgeEnabled(true)).
		withContextOptions(ctxOpts...).
		run(func(
			t *testing.T,
			vm fvm.VM,
			chain flow.Chain,
			ctx fvm.Context,
			snapshotTree snapshot.SnapshotTree,
		) {

			sc := systemcontracts.SystemContractsForChain(chain.ChainID())

			env := sc.AsTemplateEnv()

			bridgeEnv := bridge.Environment{
				CrossVMNFTAddress:                     env.ServiceAccountAddress,
				CrossVMTokenAddress:                   env.ServiceAccountAddress,
				FlowEVMBridgeHandlerInterfacesAddress: env.ServiceAccountAddress,
				IBridgePermissionsAddress:             env.ServiceAccountAddress,
				ICrossVMAddress:                       env.ServiceAccountAddress,
				ICrossVMAssetAddress:                  env.ServiceAccountAddress,
				IEVMBridgeNFTMinterAddress:            env.ServiceAccountAddress,
				IEVMBridgeTokenMinterAddress:          env.ServiceAccountAddress,
				IFlowEVMNFTBridgeAddress:              env.ServiceAccountAddress,
				IFlowEVMTokenBridgeAddress:            env.ServiceAccountAddress,
				FlowEVMBridgeAddress:                  env.ServiceAccountAddress,
				FlowEVMBridgeAccessorAddress:          env.ServiceAccountAddress,
				FlowEVMBridgeConfigAddress:            env.ServiceAccountAddress,
				FlowEVMBridgeHandlersAddress:          env.ServiceAccountAddress,
				FlowEVMBridgeNFTEscrowAddress:         env.ServiceAccountAddress,
				FlowEVMBridgeResolverAddress:          env.ServiceAccountAddress,
				FlowEVMBridgeTemplatesAddress:         env.ServiceAccountAddress,
				FlowEVMBridgeTokenEscrowAddress:       env.ServiceAccountAddress,
				FlowEVMBridgeUtilsAddress:             env.ServiceAccountAddress,
				ArrayUtilsAddress:                     env.ServiceAccountAddress,
				ScopedFTProvidersAddress:              env.ServiceAccountAddress,
				SerializeAddress:                      env.ServiceAccountAddress,
				SerializeMetadataAddress:              env.ServiceAccountAddress,
				StringUtilsAddress:                    env.ServiceAccountAddress,
			}

			// Create an account private key.
			privateKey, err := testutil.GenerateAccountPrivateKey()
			require.NoError(t, err)

			// Create accounts with the provided private
			// key and the root account.
			snapshotTree, accounts, err := testutil.CreateAccounts(
				vm,
				snapshotTree,
				[]flow.AccountPrivateKey{privateKey},
				chain)
			require.NoError(t, err)

			txBody := blueprints.TransferFlowTokenTransaction(env, chain.ServiceAddress(), accounts[0], "2.0")

			err = testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			executionSnapshot, output, err := vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)

			require.NoError(t, err)
			require.NoError(t, output.Err)

			snapshotTree = snapshotTree.Append(executionSnapshot)

			// Deploy the ExampleNFT contract
			nftContract := contracts.ExampleNFT(env)
			nftContractName := "ExampleNFT"
			txBody = blueprints.DeployContractTransaction(
				accounts[0],
				nftContract,
				nftContractName,
			)

			err = testutil.SignTransaction(txBody, accounts[0], privateKey, 0)
			require.NoError(t, err)

			executionSnapshot, output, err = vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)

			require.NoError(t, err)
			require.NoError(t, output.Err)

			snapshotTree = snapshotTree.Append(executionSnapshot)

			// Onboard the Non-Fungible Token Type
			typeToOnboard := "A." + accounts[0].String() + "." + nftContractName + ".NFT"

			txBody = blueprints.OnboardToBridgeByTypeIDTransaction(env, bridgeEnv, accounts[0], typeToOnboard)

			err = testutil.SignTransaction(txBody, accounts[0], privateKey, 1)
			require.NoError(t, err)

			executionSnapshot, output, err = vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)

			require.NoError(t, err)
			require.NoError(t, output.Err)
			require.Len(t, output.Events, 7)
			for _, event := range output.Events {
				if strings.Contains(string(event.Type), "Onboarded") {
					// decode the event payload
					data, _ := ccf.Decode(nil, event.Payload)
					// get the contractAddress field from the event
					typeOnboarded := cadence.SearchFieldByName(
						data.(cadence.Event),
						"type",
					).(cadence.String)

					require.Equal(t, typeToOnboard, typeOnboarded.String()[1:len(typeOnboarded)+1])
				}
			}

			snapshotTree = snapshotTree.Append(executionSnapshot)

			// Create COA in the new account
			txBody = blueprints.CreateCOATransaction(env, bridgeEnv, accounts[0])
			err = testutil.SignTransaction(txBody, accounts[0], privateKey, 2)
			require.NoError(t, err)

			executionSnapshot, output, err = vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)

			require.NoError(t, err)
			require.NoError(t, output.Err)

			snapshotTree = snapshotTree.Append(executionSnapshot)

			// Mint an NFT
			txBody = flow.NewTransactionBody().
				SetScript([]byte(fmt.Sprintf(
					`
						import NonFungibleToken from 0x%s
						import ExampleNFT from 0x%s
						import MetadataViews from 0x%s
						import FungibleToken from 0x%s

						transaction {

							/// local variable for storing the minter reference
							let minter: &ExampleNFT.NFTMinter

							/// Reference to the receiver's collection
							let recipientCollectionRef: &{NonFungibleToken.Receiver}

							prepare(signer: auth(BorrowValue) &Account) {

								let collectionData = ExampleNFT.resolveContractView(resourceType: nil, viewType: Type<MetadataViews.NFTCollectionData>()) as! MetadataViews.NFTCollectionData?
									?? panic("Could not resolve NFTCollectionData view. The ExampleNFT contract needs to implement the NFTCollectionData Metadata view in order to execute this transaction")

								// borrow a reference to the NFTMinter resource in storage
								self.minter = signer.storage.borrow<&ExampleNFT.NFTMinter>(from: ExampleNFT.MinterStoragePath)
									?? panic("The signer does not store an ExampleNFT.Minter object at the path "
											 .concat(ExampleNFT.MinterStoragePath.toString())
											 .concat("The signer must initialize their account with this minter resource first!"))

								// Borrow the recipient's public NFT collection reference
								self.recipientCollectionRef = getAccount(0x%s).capabilities.borrow<&{NonFungibleToken.Receiver}>(collectionData.publicPath)
									?? panic("The recipient does not have a NonFungibleToken Receiver at "
											.concat(collectionData.publicPath.toString())
											.concat(" that is capable of receiving an NFT.")
											.concat("The recipient must initialize their account with this collection and receiver first!"))
							}

							execute {
								// Mint the NFT and deposit it to the recipient's collection
								let mintedNFT <- self.minter.mintNFT(
									name: "BridgeTestNFT",
									description: "",
									thumbnail: "",
									royalties: []
								)
								self.recipientCollectionRef.deposit(token: <-mintedNFT)
							}
						}
			`,
					env.NonFungibleTokenAddress, accounts[0].String(), env.NonFungibleTokenAddress, env.FungibleTokenAddress, accounts[0].String(),
				))).AddAuthorizer(accounts[0])

			err = testutil.SignTransaction(txBody, accounts[0], privateKey, 3)
			require.NoError(t, err)

			executionSnapshot, output, err = vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)
			require.NoError(t, err)
			require.NoError(t, output.Err)

			snapshotTree = snapshotTree.Append(executionSnapshot)
			id := cadence.UInt64(0)

			for _, event := range output.Events {
				if strings.Contains(string(event.Type), "Minted") {
					// decode the event payload
					data, _ := ccf.Decode(nil, event.Payload)
					// get the contractAddress field from the event
					id = cadence.SearchFieldByName(
						data.(cadence.Event),
						"id",
					).(cadence.UInt64)
				}
			}

			// Bridge the NFT to EVM
			txBody = blueprints.BridgeNFTToEVMTransaction(env, bridgeEnv, accounts[0], typeToOnboard, id)

			err = testutil.SignTransaction(txBody, accounts[0], privateKey, 4)
			require.NoError(t, err)

			executionSnapshot, output, err = vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)

			require.NoError(t, err)
			require.NoError(t, output.Err)

			snapshotTree = snapshotTree.Append(executionSnapshot)

			// Confirm that the NFT is escrowed
			script := blueprints.GetIsNFTInEscrowScript(env, bridgeEnv)

			arguments := []cadence.Value{
				cadence.String(typeToOnboard),
				id,
			}

			encodedArguments := make([][]byte, 0, len(arguments))
			for _, argument := range arguments {
				encodedArguments = append(encodedArguments, jsoncdc.MustEncode(argument))
			}

			_, output, err = vm.Run(
				ctx,
				fvm.Script(script).
					WithArguments(encodedArguments...),
				snapshotTree)
			require.NoError(t, err)
			require.NoError(t, output.Err)

			result := output.Value.(cadence.Bool)
			require.Equal(t, cadence.Bool(true), result)

			id256 := cadence.NewUInt256(uint(id))

			// Bridge the NFT back to Cadence
			txBody = blueprints.BridgeNFTFromEVMTransaction(env, bridgeEnv, accounts[0], typeToOnboard, id256)

			err = testutil.SignTransaction(txBody, accounts[0], privateKey, 5)
			require.NoError(t, err)

			executionSnapshot, output, err = vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)

			require.NoError(t, err)
			require.NoError(t, output.Err)

			snapshotTree = snapshotTree.Append(executionSnapshot)

			// Confirm that the NFT is no longer escrowed

			_, output, err = vm.Run(
				ctx,
				fvm.Script(script).
					WithArguments(encodedArguments...),
				snapshotTree)
			require.NoError(t, err)
			require.NoError(t, output.Err)

			result = output.Value.(cadence.Bool)
			require.Equal(t, cadence.Bool(false), result)
		}),
	)
}

func TestAccountCapabilitiesGetEntitledRejection(t *testing.T) {

	// Note: This cannot be tested anymore using a transaction,
	// because publish method also aborts when trying to publish an entitled capability.
	// Therefore, test the functionality of the `ValidateAccountCapabilitiesGet` function.

	t.Run("entitled capability", func(t *testing.T) {

		env := environment.NewScriptEnv(
			context.TODO(),
			tracing.NewMockTracerSpan(),
			environment.DefaultEnvironmentParams(),
			nil,
		)

		valid, err := env.ValidateAccountCapabilitiesGet(
			nil,
			interpreter.EmptyLocationRange,
			interpreter.AddressValue(common.ZeroAddress),
			interpreter.NewUnmeteredPathValue(common.PathDomainPublic, "dummy_value"),
			sema.NewReferenceType(
				nil,
				sema.NewEntitlementSetAccess(
					[]*sema.EntitlementType{
						sema.MutateType,
					},
					sema.Conjunction,
				),
				sema.IntType,
			),
			nil,
		)
		assert.NoError(t, err)
		assert.False(t, valid)
	})

	t.Run("non-entitled capability", func(t *testing.T) {

		env := environment.NewScriptEnv(
			context.TODO(),
			tracing.NewMockTracerSpan(),
			environment.DefaultEnvironmentParams(),
			nil,
		)

		valid, err := env.ValidateAccountCapabilitiesGet(
			nil,
			interpreter.EmptyLocationRange,
			interpreter.AddressValue(common.ZeroAddress),
			interpreter.NewUnmeteredPathValue(common.PathDomainPublic, "dummy_value"),
			sema.NewReferenceType(
				nil,
				sema.UnauthorizedAccess,
				sema.IntType,
			),
			nil,
		)
		assert.NoError(t, err)
		assert.True(t, valid)
	})
}

func TestAccountCapabilitiesPublishEntitledRejection(t *testing.T) {

	t.Run("entitled capability", newVMTest().
		run(func(
			t *testing.T,
			vm fvm.VM,
			chain flow.Chain,
			ctx fvm.Context,
			snapshotTree snapshot.SnapshotTree,
		) {

			serviceAddress := chain.ServiceAddress()
			txBody := flow.NewTransactionBody().
				SetScript([]byte(`
					transaction {
                        prepare(signer: auth(Capabilities, Storage) &Account) {
                            signer.storage.save(42, to: /storage/number)
                            let cap = signer.capabilities.storage.issue<auth(Insert) &Int>(/storage/number)
                            signer.capabilities.publish(cap, at: /public/number)
                        }
					}
				`)).
				AddAuthorizer(serviceAddress).
				SetProposalKey(serviceAddress, 0, 0).
				SetPayer(serviceAddress)

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			_, output, err := vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)

			require.NoError(t, err)

			var publishingError *interpreter.EntitledCapabilityPublishingError
			require.ErrorAs(t, output.Err, &publishingError)
		}),
	)

	t.Run("non entitled capability", newVMTest().
		run(func(
			t *testing.T,
			vm fvm.VM,
			chain flow.Chain,
			ctx fvm.Context,
			snapshotTree snapshot.SnapshotTree,
		) {

			serviceAddress := chain.ServiceAddress()
			txBody := flow.NewTransactionBody().
				SetScript([]byte(`
					transaction {
                        prepare(signer: auth(Capabilities, Storage) &Account) {
                            signer.storage.save(42, to: /storage/number)
                            let cap = signer.capabilities.storage.issue<&Int>(/storage/number)
                            signer.capabilities.publish(cap, at: /public/number)
                        }
					}
				`)).
				AddAuthorizer(serviceAddress).
				SetProposalKey(serviceAddress, 0, 0).
				SetPayer(serviceAddress)

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			_, output, err := vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)

			require.NoError(t, err)
			require.NoError(t, output.Err)
		}),
	)
}

func TestCrypto(t *testing.T) {
	t.Parallel()

	const chainID = flow.Testnet

	test := func(t *testing.T, importDecl string) {

		chain, vm := createChainAndVm(chainID)

		ctx := fvm.NewContext(
			fvm.WithChain(chain),
			fvm.WithCadenceLogging(true),
		)

		script := []byte(fmt.Sprintf(
			`
              %s

              access(all)
              fun main(
                rawPublicKeys: [String],
                weights: [UFix64],
                domainSeparationTag: String,
                signatures: [String],
                toAddress: Address,
                fromAddress: Address,
                amount: UFix64
              ): Bool {
                let keyList = Crypto.KeyList()

                var i = 0
                for rawPublicKey in rawPublicKeys {
                  keyList.add(
                    PublicKey(
                      publicKey: rawPublicKey.decodeHex(),
                      signatureAlgorithm: SignatureAlgorithm.ECDSA_P256
                    ),
                    hashAlgorithm: HashAlgorithm.SHA3_256,
                    weight: weights[i],
                  )
                  i = i + 1
                }

                let signatureSet: [Crypto.KeyListSignature] = []

                var j = 0
                for signature in signatures {
                  signatureSet.append(
                    Crypto.KeyListSignature(
                      keyIndex: j,
                      signature: signature.decodeHex()
                    )
                  )
                  j = j + 1
                }

                // assemble the same message in cadence
                let message = toAddress.toBytes()
                  .concat(fromAddress.toBytes())
                  .concat(amount.toBigEndianBytes())

                return keyList.verify(
                  signatureSet: signatureSet,
                  signedData: message,
                  domainSeparationTag: domainSeparationTag
                )
              }
            `,
			importDecl,
		))

		accountKeys := test.AccountKeyGenerator()

		// create the keys
		keyAlice, signerAlice := accountKeys.NewWithSigner()
		keyBob, signerBob := accountKeys.NewWithSigner()

		// create the message that will be signed
		addresses := test.AddressGenerator()

		toAddress := cadence.Address(addresses.New())
		fromAddress := cadence.Address(addresses.New())

		amount, err := cadence.NewUFix64("100.00")
		require.NoError(t, err)

		var message []byte
		message = append(message, toAddress.Bytes()...)
		message = append(message, fromAddress.Bytes()...)
		message = append(message, amount.ToBigEndianBytes()...)

		// sign the message with Alice and Bob
		signatureAlice, err := flowsdk.SignUserMessage(signerAlice, message)
		require.NoError(t, err)

		signatureBob, err := flowsdk.SignUserMessage(signerBob, message)
		require.NoError(t, err)

		publicKeys := cadence.NewArray([]cadence.Value{
			cadence.String(hex.EncodeToString(keyAlice.PublicKey.Encode())),
			cadence.String(hex.EncodeToString(keyBob.PublicKey.Encode())),
		})

		// each signature has half weight
		weightAlice, err := cadence.NewUFix64("0.5")
		require.NoError(t, err)

		weightBob, err := cadence.NewUFix64("0.5")
		require.NoError(t, err)

		weights := cadence.NewArray([]cadence.Value{
			weightAlice,
			weightBob,
		})

		signatures := cadence.NewArray([]cadence.Value{
			cadence.String(hex.EncodeToString(signatureAlice)),
			cadence.String(hex.EncodeToString(signatureBob)),
		})

		domainSeparationTag := cadence.String("FLOW-V0.0-user")

		arguments := []cadence.Value{
			publicKeys,
			weights,
			domainSeparationTag,
			signatures,
			toAddress,
			fromAddress,
			amount,
		}

		encodedArguments := make([][]byte, 0, len(arguments))
		for _, argument := range arguments {
			encodedArguments = append(encodedArguments, jsoncdc.MustEncode(argument))
		}

		snapshotTree := testutil.RootBootstrappedLedger(vm, ctx)

		_, output, err := vm.Run(
			ctx,
			fvm.Script(script).
				WithArguments(encodedArguments...),
			snapshotTree)
		require.NoError(t, err)

		require.NoError(t, output.Err)

		result := output.Value

		assert.Equal(t,
			cadence.NewBool(true),
			result,
		)
	}

	t.Run("identifier location", func(t *testing.T) {
		t.Parallel()

		test(t, "import Crypto")
	})

	t.Run("address location", func(t *testing.T) {
		t.Parallel()

		sc := systemcontracts.SystemContractsForChain(chainID)
		cryptoContractAddress := sc.Crypto.Address.HexWithPrefix()

		test(t, fmt.Sprintf("import Crypto from %s", cryptoContractAddress))
	})
}

func Test_BlockHashListShouldWriteOnPush(t *testing.T) {

	chain := flow.Emulator.Chain()
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())

	push := func(bhl *handler.BlockHashList, height uint64) {
		buffer := make([]byte, 32)
		pos := 0

		// encode height as block hash
		binary.BigEndian.PutUint64(buffer[pos:], height)
		err := bhl.Push(height, [32]byte(buffer))
		require.NoError(t, err)
	}

	t.Run("block hash list write on push", newVMTest().
		withContextOptions(
			fvm.WithChain(chain),
			fvm.WithEVMEnabled(true),
		).
		run(func(
			t *testing.T,
			vm fvm.VM,
			chain flow.Chain,
			ctx fvm.Context,
			snapshotTree snapshot.SnapshotTree,
		) {
			capacity := 256

			// for the setup we make sure all the block hash list buckets exist

			ts := state.NewTransactionState(snapshotTree, state.DefaultParameters())
			accounts := environment.NewAccounts(ts)
			envMeter := environment.NewMeter(ts)

			valueStore := environment.NewValueStore(
				tracing.NewMockTracerSpan(),
				envMeter,
				accounts,
			)

			bhl, err := handler.NewBlockHashList(valueStore, sc.EVMStorage.Address, capacity)
			require.NoError(t, err)

			// fill the block hash list
			height := uint64(0)
			for ; height < uint64(capacity); height++ {
				push(bhl, height)
			}

			es, err := ts.FinalizeMainTransaction()
			require.NoError(t, err)
			snapshotTree = snapshotTree.Append(es)

			// end of test setup

			ts = state.NewTransactionState(snapshotTree, state.DefaultParameters())
			accounts = environment.NewAccounts(ts)
			envMeter = environment.NewMeter(ts)

			valueStore = environment.NewValueStore(
				tracing.NewMockTracerSpan(),
				envMeter,
				accounts,
			)

			bhl, err = handler.NewBlockHashList(valueStore, sc.EVMStorage.Address, capacity)
			require.NoError(t, err)

			// after we push the changes should be applied and the first block hash in the bucket should be capacity+1 instead of 0
			push(bhl, height)

			es, err = ts.FinalizeMainTransaction()
			require.NoError(t, err)

			// the write set should have both block metadata and block hash list bucket
			require.Len(t, es.WriteSet, 2)
			newBlockHashListBucket, ok := es.WriteSet[flow.NewRegisterID(sc.EVMStorage.Address, "BlockHashListBucket0")]
			require.True(t, ok)
			// full expected block hash list bucket split by individual block hashes
			// first block hash is the capacity+1 instead of 0 (00 00 00 00 00 00 01 00)
			expectedBlockHashListBucket, err := hex.DecodeString(
				"0000000000000100000000000000000000000000000000000000000000000000" +
					"0000000000000001000000000000000000000000000000000000000000000000" +
					"0000000000000002000000000000000000000000000000000000000000000000" +
					"0000000000000003000000000000000000000000000000000000000000000000" +
					"0000000000000004000000000000000000000000000000000000000000000000" +
					"0000000000000005000000000000000000000000000000000000000000000000" +
					"0000000000000006000000000000000000000000000000000000000000000000" +
					"0000000000000007000000000000000000000000000000000000000000000000" +
					"0000000000000008000000000000000000000000000000000000000000000000" +
					"0000000000000009000000000000000000000000000000000000000000000000" +
					"000000000000000a000000000000000000000000000000000000000000000000" +
					"000000000000000b000000000000000000000000000000000000000000000000" +
					"000000000000000c000000000000000000000000000000000000000000000000" +
					"000000000000000d000000000000000000000000000000000000000000000000" +
					"000000000000000e000000000000000000000000000000000000000000000000" +
					"000000000000000f000000000000000000000000000000000000000000000000")
			require.NoError(t, err)
			require.Equal(t, expectedBlockHashListBucket, newBlockHashListBucket)
		}))
}
