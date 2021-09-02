package fvm_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/engine/execution/computation/computer"
	exeState "github.com/onflow/flow-go/engine/execution/state"
	bootstrapexec "github.com/onflow/flow-go/engine/execution/state/bootstrap"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/programs"
	completeLedger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal/fixtures"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
)

type vmTestContext struct {
	chain                   flow.Chain
	serviceAccountSeqNumber uint64
}

func (c *vmTestContext) serviceAccountSequenceNumber() uint64 {
	c.serviceAccountSeqNumber = c.serviceAccountSeqNumber + 1
	return c.serviceAccountSeqNumber - 1
}

// BenchmarkRuntimeEmptyTransaction simulates executing blocks with `transactionsPerBlock`
// where each transaction is an empty transaction
func BenchmarkRuntimeTransaction(b *testing.B) {
	transactionsPerBlock := 10

	chain := flow.Testnet.Chain()

	benchTransaction := func(b *testing.B, tx string) {
		tctx := &vmTestContext{
			chain: flow.Testnet.Chain(),
		}

		executeBlocks := prepareExecutionEnv(b, tctx.chain)

		btx := []byte(tx)

		b.ResetTimer() // setup done, lets start measuring
		for i := 0; i < b.N; i++ {
			transactions := make([]*flow.TransactionBody, transactionsPerBlock)
			for j := 0; j < transactionsPerBlock; j++ {

				txBody := flow.NewTransactionBody().
					SetScript(btx).
					AddAuthorizer(tctx.chain.ServiceAddress()).
					SetProposalKey(tctx.chain.ServiceAddress(), 0, tctx.serviceAccountSequenceNumber()).
					SetPayer(tctx.chain.ServiceAddress())

				err := testutil.SignEnvelope(txBody, tctx.chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
				require.NoError(b, err)

				transactions[j] = txBody
			}

			computationResult := executeBlocks([][]*flow.TransactionBody{transactions})
			for j := 0; j < transactionsPerBlock; j++ {
				require.Empty(b, computationResult.TransactionResults[j].ErrorMessage)
			}
		}
	}

	templateTx := func(prepare string) string {
		return fmt.Sprintf(`
import FungibleToken from 0x%s
import FlowToken from 0x%s

transaction(){
	prepare(signer: AuthAccount){
		var i = 0
		while i < 100 {
			i = i + 1
%s
		}
	}
}`, fvm.FungibleTokenAddress(chain), fvm.FlowTokenAddress(chain), prepare)
	}

	b.Run("reference tx", func(b *testing.B) {
		benchTransaction(b, templateTx(""))
	})
	b.Run("log empty string", func(b *testing.B) {
		benchTransaction(b, templateTx(`log("")`))
	})
	b.Run("log long string", func(b *testing.B) {
		benchTransaction(b, templateTx(`log("0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000")`))
		// 100 character string
	})
	b.Run("convert int to string", func(b *testing.B) {
		benchTransaction(b, templateTx(`i.toString()`))
	})
	b.Run("convert int to string and concatenate it", func(b *testing.B) {
		benchTransaction(b, templateTx(`"x".concat(i.toString())`))
	})
	b.Run("get signer address", func(b *testing.B) {
		benchTransaction(b, templateTx(`signer.address`))
	})
	b.Run("get public account", func(b *testing.B) {
		benchTransaction(b, templateTx(`getAccount(signer.address)`))
	})
	b.Run("get account and get balance", func(b *testing.B) {
		benchTransaction(b, templateTx(`getAccount(signer.address).balance`))
	})
	b.Run("get account and get available balance", func(b *testing.B) {
		benchTransaction(b, templateTx(`getAccount(signer.address).availableBalance`))
	})
	b.Run("get account and get storage used", func(b *testing.B) {
		benchTransaction(b, templateTx(`getAccount(signer.address).storageUsed`))
	})
	b.Run("get account and get storage capacity", func(b *testing.B) {
		benchTransaction(b, templateTx(`getAccount(signer.address).storageCapacity`))
	})
	b.Run("get signer vault", func(b *testing.B) {
		benchTransaction(b, templateTx(`let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)!`))
	})
	b.Run("get signer receiver", func(b *testing.B) {
		benchTransaction(b, templateTx(`let receiverRef =  getAccount(signer.address)
				.getCapability(/public/flowTokenReceiver)
				.borrow<&{FungibleToken.Receiver}>()!`))
	})
	b.Run("transfer tokens", func(b *testing.B) {
		benchTransaction(b, templateTx(`
			let receiverRef =  getAccount(signer.address)
				.getCapability(/public/flowTokenReceiver)
				.borrow<&{FungibleToken.Receiver}>()!
			
			let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)!

			receiverRef.deposit(from: <-vaultRef.withdraw(amount: 0.00001))
			`))
	})
	b.Run("load and save empty string on signers address", func(b *testing.B) {
		benchTransaction(b, templateTx(`
				signer.load<String>(from: /storage/testpath)
				signer.save("", to: /storage/testpath)
			`))
	})
	b.Run("create new account", func(b *testing.B) {
		benchTransaction(b, templateTx(`let acct = AuthAccount(payer: signer)`))
	})
	// TODO:
	// create account and add key
	// create account and add key and remove a key
	// emit event
	// hash something
}

// BenchmarkRuntimeFungibleTokenTransfer simulates executing blocks with `transactionsPerBlock`
// where each transaction transfers `momentsPerTransaction` moments (NFTs)
func BenchmarkRuntimeFungibleTokenTransfer(b *testing.B) {
	transactionsPerBlock := 10

	tctx := &vmTestContext{
		chain: flow.Testnet.Chain(),
	}

	executeBlocks := prepareExecutionEnv(b, tctx.chain)

	// Create an account private key.
	privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
	require.NoError(b, err)

	// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
	accounts, err := createManyAccounts(b, tctx, executeBlocks, privateKeys)
	require.NoError(b, err)

	// Transfer NFTs
	transferTx := []byte(fmt.Sprintf(`
import FungibleToken from 0x%s
import FlowToken from 0x%s

transaction(amount: UFix64, to: Address) {
    let sentVault: @FungibleToken.Vault

    prepare(signer: AuthAccount) {
        let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)
			?? panic("Could not borrow reference to the owner's Vault!")
        self.sentVault <- vaultRef.withdraw(amount: amount)
    }

    execute {
        let receiverRef =  getAccount(to)
            .getCapability(/public/flowTokenReceiver)
            .borrow<&{FungibleToken.Receiver}>()
			?? panic("Could not borrow receiver reference to the recipient's Vault")
        receiverRef.deposit(from: <-self.sentVault)
    }
}
	`, fvm.FungibleTokenAddress(tctx.chain), fvm.FlowTokenAddress(tctx.chain)))

	encodedAddress, err := jsoncdc.Encode(cadence.BytesToAddress(accounts[0].Bytes()))
	require.NoError(b, err)

	b.ResetTimer() // setup done, lets start measuring
	for i := 0; i < b.N; i++ {
		transactions := make([]*flow.TransactionBody, transactionsPerBlock)
		for j := 0; j < transactionsPerBlock; j++ {

			txBody := flow.NewTransactionBody().
				SetScript(transferTx).
				SetProposalKey(tctx.chain.ServiceAddress(), 0, tctx.serviceAccountSequenceNumber()).
				AddAuthorizer(tctx.chain.ServiceAddress()).
				AddArgument(jsoncdc.MustEncode(cadence.UFix64(1))).
				AddArgument(encodedAddress).
				SetPayer(tctx.chain.ServiceAddress())

			err = testutil.SignEnvelope(txBody, tctx.chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
			require.NoError(b, err)

			transactions[j] = txBody
		}

		computationResult := executeBlocks([][]*flow.TransactionBody{transactions})
		for j := 0; j < transactionsPerBlock; j++ {
			require.Empty(b, computationResult.TransactionResults[j].ErrorMessage)
		}
	}
}

// BenchmarkRuntimeTopShotBatchTransfer simulates executing blocks with `transactionsPerBlock`
// where each transaction transfers `momentsPerTransaction` moments (NFTs)
func BenchmarkRuntimeTopShotBatchTransfer(b *testing.B) {
	transactionsPerBlock := 10
	momentsPerTransaction := 10

	tctx := &vmTestContext{
		chain: flow.Testnet.Chain(),
	}

	executeBlocks := prepareExecutionEnv(b, tctx.chain)

	// Create an account private key.
	privateKeys, err := testutil.GenerateAccountPrivateKeys(3)
	require.NoError(b, err)

	// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
	accounts, err := createManyAccounts(b, tctx, executeBlocks, privateKeys)
	require.NoError(b, err)

	// deploy NFT
	nftAccount := accounts[0]
	nftAccountPK := privateKeys[0]
	deployNFT(b, tctx, executeBlocks, nftAccount, nftAccountPK)

	// deploy TopShot
	topShotAccount := accounts[1]
	topShotAccountPK := privateKeys[1]
	deployTopShot(b, tctx, executeBlocks, topShotAccount, topShotAccountPK, nftAccount)

	// fund all accounts so not to run into storage problems
	fundAccounts(b, tctx, executeBlocks, cadence.UFix64(10_0000_0000), nftAccount, topShotAccount, accounts[2])

	// mint TopShot moments
	mintScript := []byte(fmt.Sprintf(`
              import TopShot from 0x%s

              transaction {

                  prepare(signer: AuthAccount) {
	                  let adminRef = signer.borrow<&TopShot.Admin>(from: /storage/TopShotAdmin)!

                      let playID = adminRef.createPlay(metadata: {"name": "Test"})
                      let setID = TopShot.nextSetID
                      adminRef.createSet(name: "Test")
                      let setRef = adminRef.borrowSet(setID: setID)
                      setRef.addPlay(playID: playID)

	                  let moments <- setRef.batchMintMoment(playID: playID, quantity: %d)

                      signer.borrow<&TopShot.Collection>(from: /storage/MomentCollection)!
                          .batchDeposit(tokens: <-moments)
                  }
              }
            `, accounts[1].Hex(), transactionsPerBlock*momentsPerTransaction*b.N))

	txBody := flow.NewTransactionBody().
		SetGasLimit(999999).
		SetScript(mintScript).
		SetProposalKey(tctx.chain.ServiceAddress(), 0, tctx.serviceAccountSequenceNumber()).
		AddAuthorizer(accounts[1]).
		SetPayer(tctx.chain.ServiceAddress())

	err = testutil.SignPayload(txBody, accounts[1], privateKeys[1])
	require.NoError(b, err)

	err = testutil.SignEnvelope(txBody, tctx.chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(b, err)

	computationResult := executeBlocks([][]*flow.TransactionBody{{txBody}})
	require.Empty(b, computationResult.TransactionResults[0].ErrorMessage)

	// Set up receiver
	setupTx := []byte(fmt.Sprintf(`
import NonFungibleToken from 0x%s
import TopShot from 0x%s

transaction {

  prepare(signer: AuthAccount) {
	  signer.save(
		 <-TopShot.createEmptyCollection(),
		 to: /storage/MomentCollection
	  )
	  signer.link<&TopShot.Collection>(
		 /public/MomentCollection,
		 target: /storage/MomentCollection
	  )
  }
}`, accounts[0].Hex(), accounts[1].Hex()))

	txBody = flow.NewTransactionBody().
		SetScript(setupTx).
		SetProposalKey(tctx.chain.ServiceAddress(), 0, tctx.serviceAccountSequenceNumber()).
		AddAuthorizer(accounts[2]).
		SetPayer(tctx.chain.ServiceAddress())

	err = testutil.SignPayload(txBody, accounts[2], privateKeys[2])
	require.NoError(b, err)

	err = testutil.SignEnvelope(txBody, tctx.chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(b, err)

	computationResult = executeBlocks([][]*flow.TransactionBody{{txBody}})
	require.Empty(b, computationResult.TransactionResults[0].ErrorMessage)

	// Transfer NFTs
	transferTx := []byte(fmt.Sprintf(`
	  import NonFungibleToken from 0x%s
	  import TopShot from 0x%s

	  transaction(momentIDs: [UInt64], recipientAddress: Address) {
	      let transferTokens: @NonFungibleToken.Collection

	      prepare(acct: AuthAccount) {
	          let ref = acct.borrow<&TopShot.Collection>(from: /storage/MomentCollection)!
	          self.transferTokens <- ref.batchWithdraw(ids: momentIDs)
	      }

	      execute {
	          // get the recipient's public account object
	          let recipient = getAccount(recipientAddress)

	          // get the Collection reference for the receiver
	          let receiverRef = recipient.getCapability(/public/MomentCollection)
	              .borrow<&{TopShot.MomentCollectionPublic}>()!

	          // deposit the NFT in the receivers collection
	          receiverRef.batchDeposit(tokens: <-self.transferTokens)
	      }
	  }
	`, accounts[0].Hex(), accounts[1].Hex()))

	encodedAddress, err := jsoncdc.Encode(cadence.BytesToAddress(accounts[2].Bytes()))
	require.NoError(b, err)

	b.ResetTimer() // setup done, lets start measuring
	for i := 0; i < b.N; i++ {
		transactions := make([]*flow.TransactionBody, transactionsPerBlock)
		for j := 0; j < transactionsPerBlock; j++ {
			cadenceValues := make([]cadence.Value, momentsPerTransaction)
			startMoment := (i*transactionsPerBlock+j)*momentsPerTransaction + 1
			for m := 0; m < momentsPerTransaction; m++ {
				cadenceValues[m] = cadence.NewUInt64(uint64(startMoment + m))
			}

			encodedArg, err := jsoncdc.Encode(
				cadence.NewArray(cadenceValues),
			)
			require.NoError(b, err)

			txBody := flow.NewTransactionBody().
				SetScript(transferTx).
				SetProposalKey(tctx.chain.ServiceAddress(), 0, tctx.serviceAccountSequenceNumber()).
				AddAuthorizer(accounts[1]).
				AddArgument(encodedArg).
				AddArgument(encodedAddress).
				SetPayer(tctx.chain.ServiceAddress())

			err = testutil.SignPayload(txBody, accounts[1], privateKeys[1])
			require.NoError(b, err)

			err = testutil.SignEnvelope(txBody, tctx.chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
			require.NoError(b, err)

			transactions[j] = txBody
		}

		computationResult = executeBlocks([][]*flow.TransactionBody{transactions})
		for j := 0; j < transactionsPerBlock; j++ {
			require.Empty(b, computationResult.TransactionResults[j].ErrorMessage)
		}
	}
}

func prepareExecutionEnv(tb testing.TB, chain flow.Chain) func(txs [][]*flow.TransactionBody) *execution.ComputationResult {
	rt := fvm.NewInterpreterRuntime()
	vm := fvm.NewVirtualMachine(rt)

	logger := zerolog.Nop()

	opts := []fvm.Option{
		fvm.WithTransactionFeesEnabled(true),
		fvm.WithAccountStorageLimit(true),
		fvm.WithChain(chain),
	}

	fvmContext :=
		fvm.NewContext(
			logger,
			opts...,
		)

	collector := metrics.NewNoopCollector()
	tracer := trace.NewNoopTracer()

	wal := &fixtures.NoopWAL{}

	ledger, err := completeLedger.NewLedger(wal, 100, collector, logger, completeLedger.DefaultPathFinderVersion)
	require.NoError(tb, err)

	bootstrapper := bootstrapexec.NewBootstrapper(logger)

	initialCommit, err := bootstrapper.BootstrapLedger(
		ledger,
		unittest.ServiceAccountPublicKey,
		chain,
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithTransactionFee(fvm.DefaultTransactionFees),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
	)

	require.NoError(tb, err)

	ledgerCommitter := committer.NewLedgerViewCommitter(ledger, tracer)

	blockComputer, err := computer.NewBlockComputer(vm, fvmContext, collector, tracer, logger, ledgerCommitter)
	require.NoError(tb, err)

	view := delta.NewView(exeState.LedgerGetRegister(ledger, initialCommit))

	return func(txs [][]*flow.TransactionBody) *execution.ComputationResult {
		executableBlock := unittest.ExecutableBlockFromTransactions(txs)
		executableBlock.StartState = &initialCommit

		computationResult, err := blockComputer.ExecuteBlock(context.Background(), executableBlock, view, programs.NewEmptyPrograms())
		require.NoError(tb, err)

		prevResultId := unittest.IdentifierFixture()

		_, _, _, err = execution.GenerateExecutionResultAndChunkDataPacks(prevResultId, initialCommit, computationResult)
		require.NoError(tb, err)

		return computationResult
	}
}

func fundAccounts(b *testing.B, tctx *vmTestContext, executeBlocks func(txs [][]*flow.TransactionBody) *execution.ComputationResult, value cadence.UFix64, accounts ...flow.Address) {
	for _, a := range accounts {

		txBody := transferTokensTx(tctx.chain)
		txBody.SetProposalKey(tctx.chain.ServiceAddress(), 0, tctx.serviceAccountSequenceNumber())
		txBody.AddArgument(jsoncdc.MustEncode(value))
		txBody.AddArgument(jsoncdc.MustEncode(cadence.BytesToAddress(a.Bytes())))
		txBody.AddAuthorizer(tctx.chain.ServiceAddress())
		txBody.SetPayer(tctx.chain.ServiceAddress())

		err := testutil.SignEnvelope(txBody, tctx.chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
		require.NoError(b, err)

		computationResult := executeBlocks([][]*flow.TransactionBody{{txBody}})
		require.Empty(b, computationResult.TransactionResults[0].ErrorMessage)
	}

}

func deployTopShot(b *testing.B, tctx *vmTestContext, executeBlocks func(txs [][]*flow.TransactionBody) *execution.ComputationResult, a flow.Address, pk flow.AccountPrivateKey, nftAddress flow.Address) {
	topShotContract := func(nftAddress flow.Address) string {
		return fmt.Sprintf(`
import NonFungibleToken from 0x%s

pub contract TopShot: NonFungibleToken {

    // -----------------------------------------------------------------------
    // TopShot contract Event definitions
    // -----------------------------------------------------------------------

    // emitted when the TopShot contract is created
    pub event ContractInitialized()

    // emitted when a new Play struct is created
    pub event PlayCreated(id: UInt32, metadata: {String:String})
    // emitted when a new series has been triggered by an admin
    pub event NewSeriesStarted(newCurrentSeries: UInt32)

    // Events for Set-Related actions
    //
    // emitted when a new Set is created
    pub event SetCreated(setID: UInt32, series: UInt32)
    // emitted when a new play is added to a set
    pub event PlayAddedToSet(setID: UInt32, playID: UInt32)
    // emitted when a play is retired from a set and cannot be used to mint
    pub event PlayRetiredFromSet(setID: UInt32, playID: UInt32, numMoments: UInt32)
    // emitted when a set is locked, meaning plays cannot be added
    pub event SetLocked(setID: UInt32)
    // emitted when a moment is minted from a set
    pub event MomentMinted(momentID: UInt64, playID: UInt32, setID: UInt32, serialNumber: UInt32)

    // events for Collection-related actions
    //
    // emitted when a moment is withdrawn from a collection
    pub event Withdraw(id: UInt64, from: Address?)
    // emitted when a moment is deposited into a collection
    pub event Deposit(id: UInt64, to: Address?)

    // emitted when a moment is destroyed
    pub event MomentDestroyed(id: UInt64)

    // -----------------------------------------------------------------------
    // TopShot contract-level fields
    // These contain actual values that are stored in the smart contract
    // -----------------------------------------------------------------------

    // Series that this set belongs to
    // Series is a concept that indicates a group of sets through time
    // Many sets can exist at a time, but only one series
    pub var currentSeries: UInt32

    // variable size dictionary of Play structs
    access(self) var playDatas: {UInt32: Play}

    // variable size dictionary of SetData structs
    access(self) var setDatas: {UInt32: SetData}

    // variable size dictionary of Set resources
    access(self) var sets: @{UInt32: Set}

    // the ID that is used to create Plays.
    // Every time a Play is created, playID is assigned
    // to the new Play's ID and then is incremented by 1.
    pub var nextPlayID: UInt32

    // the ID that is used to create Sets. Every time a Set is created
    // setID is assigned to the new set's ID and then is incremented by 1.
    pub var nextSetID: UInt32

    // the total number of Top shot moment NFTs that have been created
    // Because NFTs can be destroyed, it doesn't necessarily mean that this
    // reflects the total number of NFTs in existence, just the number that
    // have been minted to date.
    // Is also used as global moment IDs for minting
    pub var totalSupply: UInt64

    // -----------------------------------------------------------------------
    // TopShot contract-level Composite Type DEFINITIONS
    // -----------------------------------------------------------------------
    // These are just definitions for types that this contract
    // and other accounts can use. These definitions do not contain
    // actual stored values, but an instance (or object) of one of these types
    // can be created by this contract that contains stored values
    // -----------------------------------------------------------------------

    // Play is a Struct that holds metadata associated
    // with a specific NBA play, like the legendary moment when
    // Ray Allen hit the 3 to tie the Heat and Spurs in the 2013 finals game 6
    // or when Lance Stephenson blew in the ear of Lebron James
    //
    // Moment NFTs will all reference a single Play as the owner of
    // its metadata. The Plays are publicly accessible, so anyone can
    // read the metadata associated with a specific play ID
    //
    pub struct Play {

        // the unique ID that the Play has
        pub let playID: UInt32

        // Stores all the metadata about the Play as a string mapping
        // This is not the long term way we will do metadata. Just a temporary
        // construct while we figure out a better way to do metadata
        //
        pub let metadata: {String: String}

        init(metadata: {String: String}) {
            pre {
                metadata.length != 0: "New Play Metadata cannot be empty"
            }
            self.playID = TopShot.nextPlayID
            self.metadata = metadata

            // increment the ID so that it isn't used again
            TopShot.nextPlayID = TopShot.nextPlayID + UInt32(1)

            emit PlayCreated(id: self.playID, metadata: metadata)
        }
    }

    // A Set is a grouping of plays that have occurred in the real world
    // that make up a related group of collectibles, like sets of baseball
    // or Magic cards.
    //
    // SetData is a struct that is stored in a public field of the contract.
    // This is to allow anyone to be able to query the constant information
    // about a set but not have the ability to modify any data in the
    // private set resource
    //
    pub struct SetData {

        // unique ID for the set
        pub let setID: UInt32

        // Name of the Set
        // ex. "Times when the Toronto Raptors choked in the playoffs"
        pub let name: String

        // Series that this set belongs to
        // Series is a concept that indicates a group of sets through time
        // Many sets can exist at a time, but only one series
        pub let series: UInt32

        init(name: String) {
            pre {
                name.length > 0: "New Set name cannot be empty"
            }
            self.setID = TopShot.nextSetID
            self.name = name
            self.series = TopShot.currentSeries

            // increment the setID so that it isn't used again
            TopShot.nextSetID = TopShot.nextSetID + UInt32(1)

            emit SetCreated(setID: self.setID, series: self.series)
        }
    }

    // Set is a resource type that contains the functions to add and remove
    // plays from a set and mint moments.
    //
    // It is stored in a private field in the contract so that
    // the admin resource can call its methods and that there can be
    // public getters for some of its fields
    //
    // The admin can add Plays to a set so that the set can mint moments
    // that reference that playdata.
    // The moments that are minted by a set will be listed as belonging to
    // the set that minted it, as well as the Play it references
    //
    // The admin can also retire plays from the set, meaning that the retired
    // play can no longer have moments minted from it.
    //
    // If the admin locks the Set, then no more plays can be added to it, but
    // moments can still be minted.
    //
    // If retireAll() and lock() are called back to back,
    // the Set is closed off forever and nothing more can be done with it
    pub resource Set {

        // unique ID for the set
        pub let setID: UInt32

        // Array of plays that are a part of this set
        // When a play is added to the set, its ID gets appended here
        // The ID does not get removed from this array when a play is retired
        pub var plays: [UInt32]

        // Indicates if a play in this set can be minted
        // A play is set to false when it is added to a set
        // to indicate that it is still active
        // When the play is retired, this is set to true and cannot be changed
        pub var retired: {UInt32: Bool}

        // Indicates if the set is currently locked
        // When a set is created, it is unlocked
        // and plays are allowed to be added to it
        // When a set is locked, plays cannot be added
        // A set can never be changed from locked to unlocked
        // The decision to lock it is final
        // If a set is locked, plays cannot be added, but
        // moments can still be minted from plays
        // that already had been added to it.
        pub var locked: Bool

        // Indicates the number of moments
        // that have been minted per play in this set
        // When a moment is minted, this value is stored in the moment to
        // show where in the play set it is so far. ex. 13 of 60
        pub var numberMintedPerPlay: {UInt32: UInt32}

        init(name: String) {
            self.setID = TopShot.nextSetID
            self.plays = []
            self.retired = {}
            self.locked = false
            self.numberMintedPerPlay = {}

            // Create a new SetData for this Set and store it in contract storage
            TopShot.setDatas[self.setID] = SetData(name: name)
        }

        // addPlay adds a play to the set
        //
        // Parameters: playID: The ID of the play that is being added
        //
        // Pre-Conditions:
        // The play needs to be an existing play
        // The set needs to be not locked
        // The play can't have already been added to the set
        //
        pub fun addPlay(playID: UInt32) {
            pre {
                TopShot.playDatas[playID] != nil: "Cannot add the Play to Set: Play doesn't exist"
                !self.locked: "Cannot add the play to the Set after the set has been locked"
                self.numberMintedPerPlay[playID] == nil: "The play has already beed added to the set"
            }

            // Add the play to the array of plays
            self.plays.append(playID)

            // Open the play up for minting
            self.retired[playID] = false

            // Initialize the moment count to zero
            self.numberMintedPerPlay[playID] = 0

            emit PlayAddedToSet(setID: self.setID, playID: playID)
        }

        // addPlays adds multiple plays to the set
        //
        // Parameters: playIDs: The IDs of the plays that are being added
        //                      as an array
        //
        pub fun addPlays(playIDs: [UInt32]) {
            for play in playIDs {
                self.addPlay(playID: play)
            }
        }

        // retirePlay retires a play from the set so that it can't mint new moments
        //
        // Parameters: playID: The ID of the play that is being retired
        //
        // Pre-Conditions:
        // The play needs to be an existing play that is currently open for minting
        //
        pub fun retirePlay(playID: UInt32) {
            pre {
                self.retired[playID] != nil: "Cannot retire the Play: Play doesn't exist in this set!"
            }

            if !self.retired[playID]! {
                self.retired[playID] = true

                emit PlayRetiredFromSet(setID: self.setID, playID: playID, numMoments: self.numberMintedPerPlay[playID]!)
            }
        }

        // retireAll retires all the plays in the set
        // Afterwards, none of the retired plays will be able to mint new moments
        //
        pub fun retireAll() {
            for play in self.plays {
                self.retirePlay(playID: play)
            }
        }

        // lock() locks the set so that no more plays can be added to it
        //
        // Pre-Conditions:
        // The set cannot already have been locked
        pub fun lock() {
            if !self.locked {
                self.locked = true
                emit SetLocked(setID: self.setID)
            }
        }

        // mintMoment mints a new moment and returns the newly minted moment
        //
        // Parameters: playID: The ID of the play that the moment references
        //
        // Pre-Conditions:
        // The play must exist in the set and be allowed to mint new moments
        //
        // Returns: The NFT that was minted
        //
        pub fun mintMoment(playID: UInt32): @NFT {
            pre {
                self.retired[playID] != nil: "Cannot mint the moment: This play doesn't exist"
                !self.retired[playID]!: "Cannot mint the moment from this play: This play has been retired"
            }

            // get the number of moments that have been minted for this play
            // to use as this moment's serial number
            let numInPlay = self.numberMintedPerPlay[playID]!

            // mint the new moment
            let newMoment: @NFT <- create NFT(serialNumber: numInPlay + UInt32(1),
                                              playID: playID,
                                              setID: self.setID)

            // Increment the count of moments minted for this play
            self.numberMintedPerPlay[playID] = numInPlay + UInt32(1)

            return <-newMoment
        }

        // batchMintMoment mints an arbitrary quantity of moments
        // and returns them as a Collection
        //
        // Parameters: playID: the ID of the play that the moments are minted for
        //             quantity: The quantity of moments to be minted
        //
        // Returns: Collection object that contains all the moments that were minted
        //
        pub fun batchMintMoment(playID: UInt32, quantity: UInt64): @Collection {
            let newCollection <- create Collection()

            var i: UInt64 = 0
            while i < quantity {
                newCollection.deposit(token: <-self.mintMoment(playID: playID))
                i = i + UInt64(1)
            }

            return <-newCollection
        }
    }

    pub struct MomentData {

        // the ID of the Set that the Moment comes from
        pub let setID: UInt32

        // the ID of the Play that the moment references
        pub let playID: UInt32

        // the place in the play that this moment was minted
        // Otherwise know as the serial number
        pub let serialNumber: UInt32

        init(setID: UInt32, playID: UInt32, serialNumber: UInt32) {
            self.setID = setID
            self.playID = playID
            self.serialNumber = serialNumber
        }

    }

    // The resource that represents the Moment NFTs
    //
    pub resource NFT: NonFungibleToken.INFT {

        // global unique moment ID
        pub let id: UInt64

        // struct of moment metadata
        pub let data: MomentData

        init(serialNumber: UInt32, playID: UInt32, setID: UInt32) {
            // Increment the global moment IDs
            TopShot.totalSupply = TopShot.totalSupply + UInt64(1)

            self.id = TopShot.totalSupply

            // set the metadata struct
            self.data = MomentData(setID: setID, playID: playID, serialNumber: serialNumber)

            emit MomentMinted(momentID: self.id, playID: playID, setID: self.data.setID, serialNumber: self.data.serialNumber)
        }

        destroy() {
            emit MomentDestroyed(id: self.id)
        }
    }

    // Admin is a special authorization resource that
    // allows the owner to perform important functions to modify the
    // various aspects of the plays, sets, and moments
    //
    pub resource Admin {

        // createPlay creates a new Play struct
        // and stores it in the plays dictionary in the TopShot smart contract
        //
        // Parameters: metadata: A dictionary mapping metadata titles to their data
        //                       example: {"Player Name": "Kevin Durant", "Height": "7 feet"}
        //                               (because we all know Kevin Durant is not 6'9")
        //
        // Returns: the ID of the new Play object
        pub fun createPlay(metadata: {String: String}): UInt32 {
            // Create the new Play
            var newPlay = Play(metadata: metadata)
            let newID = newPlay.playID

            // Store it in the contract storage
            TopShot.playDatas[newID] = newPlay

            return newID
        }

        // createSet creates a new Set resource and returns it
        // so that the caller can store it in their account
        //
        // Parameters: name: The name of the set
        //             series: The series that the set belongs to
        //
        pub fun createSet(name: String) {
            // Create the new Set
            var newSet <- create Set(name: name)

            TopShot.sets[newSet.setID] <-! newSet
        }

        // borrowSet returns a reference to a set in the TopShot
        // contract so that the admin can call methods on it
        //
        // Parameters: setID: The ID of the set that you want to
        // get a reference to
        //
        // Returns: A reference to the set with all of the fields
        // and methods exposed
        //
        pub fun borrowSet(setID: UInt32): &Set {
            pre {
                TopShot.sets[setID] != nil: "Cannot borrow Set: The Set doesn't exist"
            }
            return &TopShot.sets[setID] as &Set
        }

        // startNewSeries ends the current series by incrementing
        // the series number, meaning that moments will be using the
        // new series number from now on
        //
        // Returns: The new series number
        //
        pub fun startNewSeries(): UInt32 {
            // end the current series and start a new one
            // by incrementing the TopShot series number
            TopShot.currentSeries = TopShot.currentSeries + UInt32(1)

            emit NewSeriesStarted(newCurrentSeries: TopShot.currentSeries)

            return TopShot.currentSeries
        }

        // createNewAdmin creates a new Admin Resource
        //
        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }
    }

    // This is the interface that users can cast their moment Collection as
    // to allow others to deposit moments into their collection
    pub resource interface MomentCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowMoment(id: UInt64): &TopShot.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow Moment reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection is a resource that every user who owns NFTs
    // will store in their account to manage their NFTS
    //
    pub resource Collection: MomentCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        // Dictionary of Moment conforming tokens
        // NFT is a resource type with a UInt64 ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        // withdraw removes an Moment from the collection and moves it to the caller
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID)
                ?? panic("Cannot withdraw: Moment does not exist in the collection")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        // batchWithdraw withdraws multiple tokens and returns them as a Collection
        pub fun batchWithdraw(ids: [UInt64]): @NonFungibleToken.Collection {
            var batchCollection <- create Collection()

            // iterate through the ids and withdraw them from the collection
            for id in ids {
                batchCollection.deposit(token: <-self.withdraw(withdrawID: id))
            }
            return <-batchCollection
        }

        // deposit takes a Moment and adds it to the collections dictionary
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @TopShot.NFT

            let id = token.id
            // add the new token to the dictionary
            let oldToken <- self.ownedNFTs[id] <- token

            if self.owner?.address != nil {
                emit Deposit(id: id, to: self.owner?.address)
            }

            destroy oldToken
        }

        // batchDeposit takes a Collection object as an argument
        // and deposits each contained NFT into this collection
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection) {
            let keys = tokens.getIDs()

            // iterate through the keys in the collection and deposit each one
            for key in keys {
                self.deposit(token: <-tokens.withdraw(withdrawID: key))
            }
            destroy tokens
        }

        // getIDs returns an array of the IDs that are in the collection
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // borrowNFT Returns a borrowed reference to a Moment in the collection
        // so that the caller can read its ID
        //
        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // borrowMoment Returns a borrowed reference to a Moment in the collection
        // so that the caller can read data and call methods from it
        // They can use this to read its setID, playID, serialNumber,
        // or any of the setData or Play Data associated with it by
        // getting the setID or playID and reading those fields from
        // the smart contract
        //
        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        pub fun borrowMoment(id: UInt64): &TopShot.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &TopShot.NFT
            } else {
                return nil
            }
        }

        // If a transaction destroys the Collection object,
        // All the NFTs contained within are also destroyed
        // Kind of like when Damien Lillard destroys the hopes and
        // dreams of the entire city of Houston
        //
        destroy() {
            destroy self.ownedNFTs
        }
    }

    // -----------------------------------------------------------------------
    // TopShot contract-level function definitions
    // -----------------------------------------------------------------------

    // createEmptyCollection creates a new, empty Collection object so that
    // a user can store it in their account storage.
    // Once they have a Collection in their storage, they are able to receive
    // Moments in transactions
    //
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <-create TopShot.Collection()
    }

    // getAllPlays returns all the plays in topshot
    //
    // Returns: An array of all the plays that have been created
    pub fun getAllPlays(): [TopShot.Play] {
        return TopShot.playDatas.values
    }

    // getPlayMetaData returns all the metadata associated with a specific play
    //
    // Parameters: playID: The id of the play that is being searched
    //
    // Returns: The metadata as a String to String mapping optional
    pub fun getPlayMetaData(playID: UInt32): {String: String}? {
        return self.playDatas[playID]?.metadata
    }

    // getPlayMetaDataByField returns the metadata associated with a
    //                        specific field of the metadata
    //                        Ex: field: "Team" will return something
    //                        like "Memphis Grizzlies"
    //
    // Parameters: playID: The id of the play that is being searched
    //             field: The field to search for
    //
    // Returns: The metadata field as a String Optional
    pub fun getPlayMetaDataByField(playID: UInt32, field: String): String? {
        // Don't force a revert if the playID or field is invalid
        if let play = TopShot.playDatas[playID] {
            return play.metadata[field]
        } else {
            return nil
        }
    }

    // getSetName returns the name that the specified set
    //            is associated with.
    //
    // Parameters: setID: The id of the set that is being searched
    //
    // Returns: The name of the set
    pub fun getSetName(setID: UInt32): String? {
        // Don't force a revert if the setID is invalid
        return TopShot.setDatas[setID]?.name
    }

    // getSetSeries returns the series that the specified set
    //              is associated with.
    //
    // Parameters: setID: The id of the set that is being searched
    //
    // Returns: The series that the set belongs to
    pub fun getSetSeries(setID: UInt32): UInt32? {
        // Don't force a revert if the setID is invalid
        return TopShot.setDatas[setID]?.series
    }

    // getSetIDsByName returns the IDs that the specified set name
    //                 is associated with.
    //
    // Parameters: setName: The name of the set that is being searched
    //
    // Returns: An array of the IDs of the set if it exists, or nil if doesn't
    pub fun getSetIDsByName(setName: String): [UInt32]? {
        var setIDs: [UInt32] = []

        // iterate through all the setDatas and search for the name
        for setData in TopShot.setDatas.values {
            if setName == setData.name {
                // if the name is found, return the ID
                setIDs.append(setData.setID)
            }
        }

        // If the name isn't found, return nil
        // Don't force a revert if the setName is invalid
        if setIDs.length == 0 {
            return nil
        } else {
            return setIDs
        }
    }

    // getPlaysInSet returns the list of play IDs that are in the set
    //
    // Parameters: setID: The id of the set that is being searched
    //
    // Returns: An array of play IDs
    pub fun getPlaysInSet(setID: UInt32): [UInt32]? {
        // Don't force a revert if the setID is invalid
        return TopShot.sets[setID]?.plays
    }

    // isEditionRetired returns a boolean that indicates if a set/play combo
    //                  (otherwise known as an edition) is retired.
    //                  If an edition is retired, it still remains in the set,
    //                  but moments can no longer be minted from it.
    //
    // Parameters: setID: The id of the set that is being searched
    //             playID: The id of the play that is being searched
    //
    // Returns: Boolean indicating if the edition is retired or not
    pub fun isEditionRetired(setID: UInt32, playID: UInt32): Bool? {
        // Don't force a revert if the set or play ID is invalid
        // remove the set from the dictionary to ket its field
        if let setToRead <- TopShot.sets.remove(key: setID) {

            let retired = setToRead.retired[playID]

            TopShot.sets[setID] <-! setToRead

            return retired
        } else {
            return nil
        }
    }

    // isSetLocked returns a boolean that indicates if a set
    //             is locked. If an set is locked,
    //             new plays can no longer be added to it,
    //             but moments can still be minted from plays
    //             that are currently in it.
    //
    // Parameters: setID: The id of the set that is being searched
    //
    // Returns: Boolean indicating if the set is locked or not
    pub fun isSetLocked(setID: UInt32): Bool? {
        // Don't force a revert if the setID is invalid
        return TopShot.sets[setID]?.locked
    }

    // getNumMomentsInEdition return the number of moments that have been
    //                        minted from a certain edition.
    //
    // Parameters: setID: The id of the set that is being searched
    //             playID: The id of the play that is being searched
    //
    // Returns: The total number of moments
    //          that have been minted from an edition
    pub fun getNumMomentsInEdition(setID: UInt32, playID: UInt32): UInt32? {
        // Don't force a revert if the set or play ID is invalid
        // remove the set from the dictionary to get its field
        if let setToRead <- TopShot.sets.remove(key: setID) {

            // read the numMintedPerPlay
            let amount = setToRead.numberMintedPerPlay[playID]

            // put the set back
            TopShot.sets[setID] <-! setToRead

            return amount
        } else {
            return nil
        }
    }

    // -----------------------------------------------------------------------
    // TopShot initialization function
    // -----------------------------------------------------------------------
    //
    init() {
        // initialize the fields
        self.currentSeries = 0
        self.playDatas = {}
        self.setDatas = {}
        self.sets <- {}
        self.nextPlayID = 1
        self.nextSetID = 1
        self.totalSupply = 0

        // Put a new Collection in storage
        self.account.save<@Collection>(<- create Collection(), to: /storage/MomentCollection)

        // create a public capability for the collection
        self.account.link<&{MomentCollectionPublic}>(/public/MomentCollection, target: /storage/MomentCollection)

        // Put the Minter in storage
        self.account.save<@Admin>(<- create Admin(), to: /storage/TopShotAdmin)

        emit ContractInitialized()
    }
}
`, nftAddress.Hex())
	}
	deployContract(b, tctx, executeBlocks, a, pk, "TopShot", topShotContract(nftAddress))
}

func deployNFT(b *testing.B, tctx *vmTestContext, executeBlocks func(txs [][]*flow.TransactionBody) *execution.ComputationResult, a flow.Address, pk flow.AccountPrivateKey) {
	const nftContract = `
pub contract interface NonFungibleToken {
    pub var totalSupply: UInt64
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub resource interface INFT {
        pub let id: UInt64
    }
    pub resource NFT: INFT {
        pub let id: UInt64
    }
    pub resource interface Provider {
        pub fun withdraw(withdrawID: UInt64): @NFT {
            post {
                result.id == withdrawID: "The ID of the withdrawn token must be the same as the requested ID"
            }
        }
    }
    pub resource interface Receiver {
        pub fun deposit(token: @NFT)
    }
    pub resource interface CollectionPublic {
        pub fun deposit(token: @NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NFT
    }
    pub resource Collection: Provider, Receiver, CollectionPublic {
        pub var ownedNFTs: @{UInt64: NFT}
        pub fun withdraw(withdrawID: UInt64): @NFT
        pub fun deposit(token: @NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NFT {
            pre {
                self.ownedNFTs[id] != nil: "NFT does not exist in the collection!"
            }
        }
    }
    pub fun createEmptyCollection(): @Collection {
        post {
            result.getIDs().length == 0: "The created collection must be empty!"
        }
    }
}`

	deployContract(b, tctx, executeBlocks, a, pk, "NonFungibleToken", nftContract)
}

func deployContract(b *testing.B, tctx *vmTestContext, executeBlocks func(txs [][]*flow.TransactionBody) *execution.ComputationResult, a flow.Address, pk flow.AccountPrivateKey, contractName string, contract string) {

	txBody := testutil.CreateContractDeploymentTransaction(
		contractName,
		contract,
		a,
		tctx.chain)

	txBody.SetProposalKey(tctx.chain.ServiceAddress(), 0, tctx.serviceAccountSequenceNumber())
	txBody.SetPayer(tctx.chain.ServiceAddress())

	err := testutil.SignPayload(txBody, a, pk)
	require.NoError(b, err)

	err = testutil.SignEnvelope(txBody, tctx.chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(b, err)

	computationResult := executeBlocks([][]*flow.TransactionBody{{txBody}})
	require.Empty(b, computationResult.TransactionResults[0].ErrorMessage)
}

func createManyAccounts(
	tb testing.TB,
	tctx *vmTestContext,
	executeBlocks func(txs [][]*flow.TransactionBody) *execution.ComputationResult,
	privateKeys []flow.AccountPrivateKey,
) ([]flow.Address, error) {
	var accounts []flow.Address

	script := []byte(`
	  transaction(publicKey: [UInt8]) {
	    prepare(signer: AuthAccount) {
	  	  let acct = AuthAccount(payer: signer)
	  	  acct.addPublicKey(publicKey)
	    }
	  }
	`)

	serviceAddress := tctx.chain.ServiceAddress()

	for _, privateKey := range privateKeys {
		accountKey := privateKey.PublicKey(fvm.AccountKeyWeightThreshold)
		encAccountKey, _ := flow.EncodeRuntimeAccountPublicKey(accountKey)
		cadAccountKey := testutil.BytesToCadenceArray(encAccountKey)
		encCadAccountKey, _ := jsoncdc.Encode(cadAccountKey)

		txBody := flow.NewTransactionBody().
			SetScript(script).
			AddArgument(encCadAccountKey).
			AddAuthorizer(serviceAddress).
			SetProposalKey(serviceAddress, 0, tctx.serviceAccountSequenceNumber()).
			SetPayer(serviceAddress)

		err := testutil.SignEnvelope(txBody, tctx.chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
		require.NoError(tb, err)

		computationResult := executeBlocks([][]*flow.TransactionBody{{txBody}})
		require.Empty(tb, computationResult.TransactionResults[0].ErrorMessage)

		var addr flow.Address

		for _, eventList := range computationResult.Events {
			for _, event := range eventList {
				if event.Type == flow.EventAccountCreated {
					data, err := jsoncdc.Decode(event.Payload)
					if err != nil {
						return nil, errors.New("error decoding events")
					}
					addr = flow.Address(data.(cadence.Event).Fields[0].(cadence.Address))
					break
				}
			}
		}
		if addr == flow.EmptyAddress {
			return nil, errors.New("no account creation event emitted")
		}
		accounts = append(accounts, addr)
	}

	return accounts, nil
}
