package fvm_test

import (
	"fmt"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestAccountBalanceReadsCanonicalVault verifies that Cadence's
// Account.balance field returns the balance of the canonical FlowToken vault
// stored at /storage/flowTokenVault, read directly from account storage by
// the FVM.
//
// It must NOT reflect the balance capability published at
// /public/flowTokenBalance: that capability is controlled by the account
// owner and can be republished to point at a different vault (or unpublished
// entirely), so protocol code cannot rely on it.
func TestAccountBalanceReadsCanonicalVault(t *testing.T) {
	t.Parallel()

	chain := flow.Testnet.Chain()
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())

	secondVaultStoragePath := "/storage/secondFlowTokenVault"
	fundingAmount := uint64(100_000_000) // 1.0 FLOW

	// accountBalance returns the account's balance as seen by Cadence's
	// Account.balance field.
	accountBalance := func(
		t *testing.T,
		vm fvm.VM,
		ctx fvm.Context,
		snapshotTree snapshot.SnapshotTree,
		address flow.Address,
	) uint64 {
		script := fvm.Script([]byte(
			`
			access(all) fun main(addr: Address): UFix64 {
				return getAccount(addr).balance
			}`,
		)).WithArguments(
			jsoncdc.MustEncode(cadence.NewAddress(address)),
		)

		_, output, err := vm.Run(ctx, script, snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)
		return uint64(output.Value.(cadence.UFix64))
	}

	// publicCapabilityBalance returns the balance seen through
	// /public/flowTokenBalance.
	publicCapabilityBalance := func(
		t *testing.T,
		vm fvm.VM,
		ctx fvm.Context,
		snapshotTree snapshot.SnapshotTree,
		address flow.Address,
	) uint64 {
		code := fmt.Appendf(nil,
			`
			import FlowToken from 0x%s

			access(all) fun main(addr: Address): UFix64 {
				let vaultRef = getAccount(addr).capabilities.borrow<&FlowToken.Vault>(/public/flowTokenBalance)
					?? panic("missing public balance capability")
				return vaultRef.balance
			}`,
			sc.FlowToken.Address.Hex(),
		)

		script := fvm.Script(code).WithArguments(
			jsoncdc.MustEncode(cadence.NewAddress(address)),
		)

		_, output, err := vm.Run(ctx, script, snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)
		return uint64(output.Value.(cadence.UFix64))
	}

	createFundedAccount := func(
		t *testing.T,
		vm fvm.VM,
		chain flow.Chain,
		ctx fvm.Context,
		snapshotTree snapshot.SnapshotTree,
	) (snapshot.SnapshotTree, flow.Address, flow.AccountPrivateKey) {
		privateKey, txBodyBuilder := testutil.CreateAccountCreationTransaction(t, chain)

		err := testutil.SignTransactionAsServiceAccount(txBodyBuilder, 0, chain)
		require.NoError(t, err)

		txBody, err := txBodyBuilder.Build()
		require.NoError(t, err)

		executionSnapshot, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

		accountCreatedEvents := filterAccountCreatedEvents(output.Events)
		require.Len(t, accountCreatedEvents, 1)

		data, err := ccf.Decode(nil, accountCreatedEvents[0].Payload)
		require.NoError(t, err)

		address := flow.Address(
			cadence.SearchFieldByName(
				data.(cadence.Event),
				"address",
			).(cadence.Address),
		)

		txBodyBuilder = transferTokensTx(chain).
			AddAuthorizer(chain.ServiceAddress()).
			AddArgument(jsoncdc.MustEncode(cadence.UFix64(fundingAmount))).
			AddArgument(jsoncdc.MustEncode(cadence.NewAddress(address))).
			SetProposalKey(chain.ServiceAddress(), 0, 1).
			SetPayer(chain.ServiceAddress())

		err = testutil.SignEnvelope(
			txBodyBuilder,
			chain.ServiceAddress(),
			unittest.ServiceAccountPrivateKey,
		)
		require.NoError(t, err)

		txBody, err = txBodyBuilder.Build()
		require.NoError(t, err)

		executionSnapshot, output, err = vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		return snapshotTree.Append(executionSnapshot), address, privateKey
	}

	newVMTest().run(func(
		t *testing.T,
		vm fvm.VM,
		chain flow.Chain,
		ctx fvm.Context,
		snapshotTree snapshot.SnapshotTree,
	) {
		snapshotTree, address, privateKey := createFundedAccount(t, vm, chain, ctx, snapshotTree)

		canonicalBalance := accountBalance(t, vm, ctx, snapshotTree, address)
		require.Equal(t, fundingAmount, publicCapabilityBalance(t, vm, ctx, snapshotTree, address))

		// Republish /public/flowTokenBalance to a second vault holding a
		// different (here: 0) balance.
		republishScript := fmt.Sprintf(
			`
			import FlowToken from 0x%s

			transaction {
				prepare(signer: auth(Storage, Capabilities) &Account) {
					let secondVault <- FlowToken.createEmptyVault(vaultType: Type<@FlowToken.Vault>())
					signer.storage.save(<-secondVault, to: %s)

					signer.capabilities.unpublish(/public/flowTokenBalance)
					let balanceCap = signer.capabilities.storage.issue<&FlowToken.Vault>(%s)
					signer.capabilities.publish(balanceCap, at: /public/flowTokenBalance)
				}
			}`,
			sc.FlowToken.Address.Hex(),
			secondVaultStoragePath,
			secondVaultStoragePath,
		)

		txBodyBuilder := flow.NewTransactionBodyBuilder().
			SetScript([]byte(republishScript)).
			AddAuthorizer(address).
			SetProposalKey(address, 0, 0).
			SetPayer(address).
			SetComputeLimit(fvm.DefaultComputationLimit)

		err := testutil.SignEnvelope(txBodyBuilder, address, privateKey)
		require.NoError(t, err)

		txBody, err := txBodyBuilder.Build()
		require.NoError(t, err)

		executionSnapshot, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

		// The public capability now sees the empty second vault...
		require.Equal(t, uint64(0), publicCapabilityBalance(t, vm, ctx, snapshotTree, address))

		// ...but Account.balance still returns the canonical vault's balance
		require.Equal(t, canonicalBalance, accountBalance(t, vm, ctx, snapshotTree, address))
		require.Equal(t, fundingAmount, accountBalance(t, vm, ctx, snapshotTree, address))
	})(t)
}

// TestAccountBalanceWithoutVault verifies that an account without a
// FlowToken vault at /storage/flowTokenVault has a balance of 0, matching the
// semantics of the previous contract-based implementation.
func TestAccountBalanceWithoutVault(t *testing.T) {
	t.Parallel()

	newVMTest().run(func(
		t *testing.T,
		vm fvm.VM,
		chain flow.Chain,
		ctx fvm.Context,
		snapshotTree snapshot.SnapshotTree,
	) {
		privateKey, txBodyBuilder := testutil.CreateAccountCreationTransaction(t, chain)

		err := testutil.SignTransactionAsServiceAccount(txBodyBuilder, 0, chain)
		require.NoError(t, err)

		txBody, err := txBodyBuilder.Build()
		require.NoError(t, err)

		executionSnapshot, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

		accountCreatedEvents := filterAccountCreatedEvents(output.Events)
		require.Len(t, accountCreatedEvents, 1)

		data, err := ccf.Decode(nil, accountCreatedEvents[0].Payload)
		require.NoError(t, err)

		address := flow.Address(
			cadence.SearchFieldByName(
				data.(cadence.Event),
				"address",
			).(cadence.Address),
		)

		sc := systemcontracts.SystemContractsForChain(chain.ChainID())

		// Remove the canonical vault entirely (and unpublish the capabilities
		// pointing at it).
		removeVaultScript := fmt.Sprintf(
			`
			import FlowToken from 0x%s

			transaction {
				prepare(signer: auth(Storage, Capabilities) &Account) {
					signer.capabilities.unpublish(/public/flowTokenBalance)
					signer.capabilities.unpublish(/public/flowTokenReceiver)
					let vault <- signer.storage.load<@FlowToken.Vault>(from: /storage/flowTokenVault)
					destroy vault
				}
			}`,
			sc.FlowToken.Address.Hex(),
		)

		txBodyBuilder = flow.NewTransactionBodyBuilder().
			SetScript([]byte(removeVaultScript)).
			AddAuthorizer(address).
			SetProposalKey(address, 0, 0).
			SetPayer(address).
			SetComputeLimit(fvm.DefaultComputationLimit)

		err = testutil.SignEnvelope(txBodyBuilder, address, privateKey)
		require.NoError(t, err)

		txBody, err = txBodyBuilder.Build()
		require.NoError(t, err)

		executionSnapshot, output, err = vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

		script := fvm.Script([]byte(
			`
			access(all) fun main(addr: Address): UFix64 {
				return getAccount(addr).balance
			}`,
		)).WithArguments(
			jsoncdc.MustEncode(cadence.NewAddress(address)),
		)

		_, output, err = vm.Run(ctx, script, snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)
		require.Equal(t, uint64(0), uint64(output.Value.(cadence.UFix64)))
	})(t)
}

// TestAccountBalanceIgnoresNonFlowTokenVault verifies that a resource stored
// at /storage/flowTokenVault that is NOT a FlowToken.Vault does NOT count toward
// Account.balance. This matches the semantics of the previous contract-based
// implementation, which borrowed the vault with an explicit
// &FlowToken.Vault type and treated a type mismatch as "no vault" (balance 0).
func TestAccountBalanceIgnoresNonFlowTokenVault(t *testing.T) {
	t.Parallel()

	newVMTest().run(func(
		t *testing.T,
		vm fvm.VM,
		chain flow.Chain,
		ctx fvm.Context,
		snapshotTree snapshot.SnapshotTree,
	) {
		// ==== Deploy a fake token contract to the service account ====
		fakeTokenContract := `
			access(all) contract FakeToken {
				access(all) resource Vault {
					access(all) var balance: UFix64

					init(balance: UFix64) {
						self.balance = balance
					}
				}

				access(all) fun createEmptyVault(): @Vault {
					return <-create Vault(balance: 0.0)
				}

				access(all) fun createVault(balance: UFix64): @Vault {
					return <-create Vault(balance: balance)
				}
			}`

		txBodyBuilder := blueprints.DeployContractTransaction(
			chain.ServiceAddress(),
			[]byte(fakeTokenContract),
			"FakeToken",
		).
			SetProposalKey(chain.ServiceAddress(), 0, 0).
			SetPayer(chain.ServiceAddress())

		err := testutil.SignEnvelope(
			txBodyBuilder,
			chain.ServiceAddress(),
			unittest.ServiceAccountPrivateKey,
		)
		require.NoError(t, err)

		txBody, err := txBodyBuilder.Build()
		require.NoError(t, err)

		executionSnapshot, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

		// ==== Create an account (gets a real FlowToken vault at setup) ====
		privateKey, txBodyBuilder := testutil.CreateAccountCreationTransaction(t, chain)

		err = testutil.SignTransactionAsServiceAccount(txBodyBuilder, 1, chain)
		require.NoError(t, err)

		txBody, err = txBodyBuilder.Build()
		require.NoError(t, err)

		executionSnapshot, output, err = vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

		accountCreatedEvents := filterAccountCreatedEvents(output.Events)
		require.Len(t, accountCreatedEvents, 1)

		data, err := ccf.Decode(nil, accountCreatedEvents[0].Payload)
		require.NoError(t, err)

		address := flow.Address(
			cadence.SearchFieldByName(
				data.(cadence.Event),
				"address",
			).(cadence.Address),
		)

		// ==== Replace the canonical vault with a fake vault that has a large
		//      balance ====
		sc := systemcontracts.SystemContractsForChain(chain.ChainID())
		swapScript := fmt.Sprintf(
			`
			import FlowToken from 0x%s
			import FakeToken from 0x%s

			transaction {
				prepare(signer: auth(Storage, Capabilities) &Account) {
					signer.capabilities.unpublish(/public/flowTokenBalance)
					signer.capabilities.unpublish(/public/flowTokenReceiver)

					let realVault <- signer.storage.load<@FlowToken.Vault>(from: /storage/flowTokenVault)
					destroy realVault

					let fakeVault <- FakeToken.createVault(balance: 1000000.0)
					signer.storage.save(<-fakeVault, to: /storage/flowTokenVault)
				}
			}`,
			sc.FlowToken.Address.Hex(),
			chain.ServiceAddress().Hex(),
		)

		txBodyBuilder = flow.NewTransactionBodyBuilder().
			SetScript([]byte(swapScript)).
			AddAuthorizer(address).
			SetProposalKey(address, 0, 0).
			SetPayer(address).
			SetComputeLimit(fvm.DefaultComputationLimit)

		err = testutil.SignEnvelope(txBodyBuilder, address, privateKey)
		require.NoError(t, err)

		txBody, err = txBodyBuilder.Build()
		require.NoError(t, err)

		executionSnapshot, output, err = vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

		// ==== Account.balance must ignore the fake vault and return 0 ====
		script := fvm.Script([]byte(
			`
			access(all) fun main(addr: Address): UFix64 {
				return getAccount(addr).balance
			}`,
		)).WithArguments(
			jsoncdc.MustEncode(cadence.NewAddress(address)),
		)

		_, output, err = vm.Run(ctx, script, snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)
		require.Equal(
			t,
			uint64(0),
			uint64(output.Value.(cadence.UFix64)),
			"a FakeToken.Vault stored at /storage/flowTokenVault must not count toward Account.balance",
		)

		// ==== Replace the fake vault with a vault of the REAL FlowToken
		//      contract, but deployed at a different address (a clone).
		//      Its type (A.<service>.FlowToken.Vault) is a FlowToken.Vault,
		//      but not the chain's canonical one. ====

		// deploy a clone of the FlowToken contract (same contract and
		// resource names, so the vault's type is FlowToken.Vault, but at a
		// different address) to the service account
		clonedFlowTokenContract := `
			access(all) contract FlowToken {
				access(all) resource Vault {
					access(all) var balance: UFix64

					init(balance: UFix64) {
						self.balance = balance
					}
				}

				access(all) resource Minter {
					access(all) fun mintTokens(amount: UFix64): @Vault {
						return <-create Vault(balance: amount)
					}
				}

				init() {
					self.account.storage.save(<-create Minter(), to: /storage/clonedFlowTokenMinter)
				}
			}`

		txBodyBuilder = blueprints.DeployContractTransaction(
			chain.ServiceAddress(),
			[]byte(clonedFlowTokenContract),
			"FlowToken",
		).
			SetProposalKey(chain.ServiceAddress(), 0, 2).
			SetPayer(chain.ServiceAddress())

		err = testutil.SignEnvelope(
			txBodyBuilder,
			chain.ServiceAddress(),
			unittest.ServiceAccountPrivateKey,
		)
		require.NoError(t, err)

		txBody, err = txBodyBuilder.Build()
		require.NoError(t, err)

		executionSnapshot, output, err = vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

		// the service account mints a large balance from the cloned contract's
		// minter, and the attacker stores the cloned vault at the canonical
		// path (the previous vault is loaded as AnyResource so the transaction
		// only needs to import the cloned FlowToken)
		cloneSwapScript := fmt.Sprintf(
			`
			import FlowToken from 0x%s

			transaction(amount: UFix64) {
				prepare(service: auth(Storage) &Account, signer: auth(Storage) &Account) {
					let previousVault <- signer.storage.load<@AnyResource>(from: /storage/flowTokenVault)
					destroy previousVault

					let minter = service.storage.borrow<&FlowToken.Minter>(from: /storage/clonedFlowTokenMinter)
						?? panic("missing cloned minter")
					let clonedVault <- minter.mintTokens(amount: amount)
					signer.storage.save(<-clonedVault, to: /storage/flowTokenVault)
				}
			}`,
			chain.ServiceAddress().Hex(),
		)

		txBodyBuilder = flow.NewTransactionBodyBuilder().
			SetScript([]byte(cloneSwapScript)).
			AddArgument(jsoncdc.MustEncode(cadence.UFix64(2_000_000_000_000_000))).
			AddAuthorizer(chain.ServiceAddress()).
			AddAuthorizer(address).
			SetProposalKey(address, 0, 1).
			SetPayer(address).
			SetComputeLimit(fvm.DefaultComputationLimit)

		// the service account is an authorizer but not the payer, so it signs
		// the payload; the attacker is payer/proposer/authorizer and signs the
		// envelope
		err = testutil.SignPayload(
			txBodyBuilder,
			chain.ServiceAddress(),
			unittest.ServiceAccountPrivateKey,
		)
		require.NoError(t, err)
		err = testutil.SignEnvelope(txBodyBuilder, address, privateKey)
		require.NoError(t, err)

		txBody, err = txBodyBuilder.Build()
		require.NoError(t, err)

		executionSnapshot, output, err = vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

		// ==== Account.balance must also ignore a FlowToken.Vault that is not
		//      the chain's canonical one ====
		_, output, err = vm.Run(ctx, script, snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)
		require.Equal(
			t,
			uint64(0),
			uint64(output.Value.(cadence.UFix64)),
			"a FlowToken.Vault from a non-canonical FlowToken deployment must not count toward Account.balance",
		)
	})(t)
}

// TestAccountBalanceIgnoresNonResourceValue verifies that a non-resource
// value stored at /storage/flowTokenVault (e.g. an Int) does not count toward
// Account.balance either. The previous contract-based implementation borrowed
// a typed &FlowToken.Vault, which evaluates to nil (balance 0) for any type
// mismatch, including non-resources.
func TestAccountBalanceIgnoresNonResourceValue(t *testing.T) {
	t.Parallel()

	newVMTest().run(func(
		t *testing.T,
		vm fvm.VM,
		chain flow.Chain,
		ctx fvm.Context,
		snapshotTree snapshot.SnapshotTree,
	) {
		privateKey, txBodyBuilder := testutil.CreateAccountCreationTransaction(t, chain)

		err := testutil.SignTransactionAsServiceAccount(txBodyBuilder, 0, chain)
		require.NoError(t, err)

		txBody, err := txBodyBuilder.Build()
		require.NoError(t, err)

		executionSnapshot, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

		accountCreatedEvents := filterAccountCreatedEvents(output.Events)
		require.Len(t, accountCreatedEvents, 1)

		data, err := ccf.Decode(nil, accountCreatedEvents[0].Payload)
		require.NoError(t, err)

		address := flow.Address(
			cadence.SearchFieldByName(
				data.(cadence.Event),
				"address",
			).(cadence.Address),
		)

		sc := systemcontracts.SystemContractsForChain(chain.ChainID())

		// Replace the canonical vault with a plain Int value.
		swapScript := fmt.Sprintf(
			`
			import FlowToken from 0x%s

			transaction {
				prepare(signer: auth(Storage, Capabilities) &Account) {
					signer.capabilities.unpublish(/public/flowTokenBalance)
					signer.capabilities.unpublish(/public/flowTokenReceiver)

					let realVault <- signer.storage.load<@FlowToken.Vault>(from: /storage/flowTokenVault)
					destroy realVault

					signer.storage.save(1, to: /storage/flowTokenVault)
				}
			}`,
			sc.FlowToken.Address.Hex(),
		)

		txBodyBuilder = flow.NewTransactionBodyBuilder().
			SetScript([]byte(swapScript)).
			AddAuthorizer(address).
			SetProposalKey(address, 0, 0).
			SetPayer(address).
			SetComputeLimit(fvm.DefaultComputationLimit)

		err = testutil.SignEnvelope(txBodyBuilder, address, privateKey)
		require.NoError(t, err)

		txBody, err = txBodyBuilder.Build()
		require.NoError(t, err)

		executionSnapshot, output, err = vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

		script := fvm.Script([]byte(
			`
			access(all) fun main(addr: Address): UFix64 {
				return getAccount(addr).balance
			}`,
		)).WithArguments(
			jsoncdc.MustEncode(cadence.NewAddress(address)),
		)

		_, output, err = vm.Run(ctx, script, snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)
		require.Equal(
			t,
			uint64(0),
			uint64(output.Value.(cadence.UFix64)),
			"a non-resource value stored at /storage/flowTokenVault must not count toward Account.balance",
		)
	})(t)
}
