package fvm_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(b, err)

		// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
		accounts, err := createManyAccounts(b, tctx, executeBlocks, privateKeys)
		require.NoError(b, err)

		deployContract(b, tctx, executeBlocks, accounts[0], privateKeys[0], "TestContract", `
access(all) contract TestContract {
	access(all) event SomeEvent()

    access(all) fun empty() {
    }

	access(all) fun emit() {
		emit SomeEvent()
    }
}
`)

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

	longString := strings.Repeat("0", 1000)

	templateTx := func(prepare string) string {
		return fmt.Sprintf(`
import FungibleToken from 0x%s
import FlowToken from 0x%s
import TestContract from 0x%s

transaction(){
	prepare(signer: AuthAccount){
		var i = 0
		while i < 100 {
			i = i + 1
%s
		}
	}
}`, fvm.FungibleTokenAddress(chain), fvm.FlowTokenAddress(chain), "754aed9de6197641", prepare)
	}

	b.Run("reference tx", func(b *testing.B) {
		benchTransaction(b, templateTx(""))
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
	b.Run("load and save long string on signers address", func(b *testing.B) {
		benchTransaction(b, templateTx(fmt.Sprintf(`
				signer.load<String>(from: /storage/testpath)
				signer.save("%s", to: /storage/testpath)
			`, longString)))
	})
	b.Run("create new account", func(b *testing.B) {
		benchTransaction(b, templateTx(`let acct = AuthAccount(payer: signer)`))
	})
	b.Run("call empty contract function", func(b *testing.B) {
		benchTransaction(b, templateTx(`TestContract.empty()`))
	})
	b.Run("emit event", func(b *testing.B) {
		benchTransaction(b, templateTx(`TestContract.emit()`))
	})
}

// BenchmarkRuntimeNFTBatchTransfer simulates executing blocks with `transactionsPerBlock`
// where each transaction transfers `testTokensPerTransaction` testTokens (NFTs)
func BenchmarkRuntimeNFTBatchTransfer(b *testing.B) {
	transactionsPerBlock := 10
	testTokensPerTransaction := 10

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

	// deploy NFT
	batchNFTAccount := accounts[1]
	batchNFTAccountPK := privateKeys[1]
	deployBatchNFT(b, tctx, executeBlocks, batchNFTAccount, batchNFTAccountPK, nftAccount)

	// fund all accounts so not to run into storage problems
	fundAccounts(b, tctx, executeBlocks, cadence.UFix64(10_0000_0000), nftAccount, batchNFTAccount, accounts[2])

	// mint NFT testTokens
	mintScript := []byte(fmt.Sprintf(`
              import BatchNFT from 0x%s

              transaction {

                  prepare(signer: AuthAccount) {
	                  let adminRef = signer.borrow<&BatchNFT.Admin>(from: /storage/BatchNFTAdmin)!

                      let playID = adminRef.createPlay(metadata: {"name": "Test"})
                      let setID = BatchNFT.nextSetID
                      adminRef.createSet(name: "Test")
                      let setRef = adminRef.borrowSet(setID: setID)
                      setRef.addPlay(playID: playID)

	                  let testTokens <- setRef.batchMintTestToken(playID: playID, quantity: %d)

                      signer.borrow<&BatchNFT.Collection>(from: /storage/TestTokenCollection)!
                          .batchDeposit(tokens: <-testTokens)
                  }
              }
            `, accounts[1].Hex(), transactionsPerBlock*testTokensPerTransaction*b.N))

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
import BatchNFT from 0x%s

transaction {

  prepare(signer: AuthAccount) {
	  signer.save(
		 <-BatchNFT.createEmptyCollection(),
		 to: /storage/TestTokenCollection
	  )
	  signer.link<&BatchNFT.Collection>(
		 /public/TestTokenCollection,
		 target: /storage/TestTokenCollection
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
	  import BatchNFT from 0x%s

	  transaction(testTokenIDs: [UInt64], recipientAddress: Address) {
	      let transferTokens: @NonFungibleToken.Collection

	      prepare(acct: AuthAccount) {
	          let ref = acct.borrow<&BatchNFT.Collection>(from: /storage/TestTokenCollection)!
	          self.transferTokens <- ref.batchWithdraw(ids: testTokenIDs)
	      }

	      execute {
	          // get the recipient's public account object
	          let recipient = getAccount(recipientAddress)

	          // get the Collection reference for the receiver
	          let receiverRef = recipient.getCapability(/public/TestTokenCollection)
	              .borrow<&{BatchNFT.TestTokenCollectionPublic}>()!

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
			cadenceValues := make([]cadence.Value, testTokensPerTransaction)
			startTestToken := (i*transactionsPerBlock+j)*testTokensPerTransaction + 1
			for m := 0; m < testTokensPerTransaction; m++ {
				cadenceValues[m] = cadence.NewUInt64(uint64(startTestToken + m))
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

	blockPrograms := programs.NewEmptyPrograms()

	return func(txs [][]*flow.TransactionBody) *execution.ComputationResult {
		executableBlock := unittest.ExecutableBlockFromTransactions(txs)
		executableBlock.StartState = &initialCommit

		computationResult, err := blockComputer.ExecuteBlock(context.Background(), executableBlock, view, blockPrograms)
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

func deployBatchNFT(b *testing.B, tctx *vmTestContext, executeBlocks func(txs [][]*flow.TransactionBody) *execution.ComputationResult, a flow.Address, pk flow.AccountPrivateKey, nftAddress flow.Address) {
	batchNFTContract := func(nftAddress flow.Address) string {
		return fmt.Sprintf(`
import NonFungibleToken from 0x%s

pub contract BatchNFT: NonFungibleToken {
    pub event ContractInitialized()
    pub event PlayCreated(id: UInt32, metadata: {String:String})
    pub event NewSeriesStarted(newCurrentSeries: UInt32)
    pub event SetCreated(setID: UInt32, series: UInt32)
    pub event PlayAddedToSet(setID: UInt32, playID: UInt32)
    pub event PlayRetiredFromSet(setID: UInt32, playID: UInt32, numTestTokens: UInt32)
    pub event SetLocked(setID: UInt32)
    pub event TestTokenMinted(testTokenID: UInt64, playID: UInt32, setID: UInt32, serialNumber: UInt32)
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event TestTokenDestroyed(id: UInt64)
    pub var currentSeries: UInt32
    access(self) var playDatas: {UInt32: Play}
    access(self) var setDatas: {UInt32: SetData}
    access(self) var sets: @{UInt32: Set}
    pub var nextPlayID: UInt32
    pub var nextSetID: UInt32
    pub var totalSupply: UInt64

    pub struct Play {
        pub let playID: UInt32
        pub let metadata: {String: String}

        init(metadata: {String: String}) {
            pre {
                metadata.length != 0: "New Play Metadata cannot be empty"
            }
            self.playID = BatchNFT.nextPlayID
            self.metadata = metadata

            BatchNFT.nextPlayID = BatchNFT.nextPlayID + UInt32(1)
            emit PlayCreated(id: self.playID, metadata: metadata)
        }
    }

    pub struct SetData {
        pub let setID: UInt32
        pub let name: String
        pub let series: UInt32
        init(name: String) {
            pre {
                name.length > 0: "New Set name cannot be empty"
            }
            self.setID = BatchNFT.nextSetID
            self.name = name
            self.series = BatchNFT.currentSeries
            BatchNFT.nextSetID = BatchNFT.nextSetID + UInt32(1)
            emit SetCreated(setID: self.setID, series: self.series)
        }
    }

    pub resource Set {
        pub let setID: UInt32
        pub var plays: [UInt32]
        pub var retired: {UInt32: Bool}
        pub var locked: Bool
        pub var numberMintedPerPlay: {UInt32: UInt32}

        init(name: String) {
            self.setID = BatchNFT.nextSetID
            self.plays = []
            self.retired = {}
            self.locked = false
            self.numberMintedPerPlay = {}

            BatchNFT.setDatas[self.setID] = SetData(name: name)
        }

        pub fun addPlay(playID: UInt32) {
            pre {
                BatchNFT.playDatas[playID] != nil: "Cannot add the Play to Set: Play doesn't exist"
                !self.locked: "Cannot add the play to the Set after the set has been locked"
                self.numberMintedPerPlay[playID] == nil: "The play has already beed added to the set"
            }

            self.plays.append(playID)
            self.retired[playID] = false
            self.numberMintedPerPlay[playID] = 0
            emit PlayAddedToSet(setID: self.setID, playID: playID)
        }

        pub fun addPlays(playIDs: [UInt32]) {
            for play in playIDs {
                self.addPlay(playID: play)
            }
        }

        pub fun retirePlay(playID: UInt32) {
            pre {
                self.retired[playID] != nil: "Cannot retire the Play: Play doesn't exist in this set!"
            }

            if !self.retired[playID]! {
                self.retired[playID] = true

                emit PlayRetiredFromSet(setID: self.setID, playID: playID, numTestTokens: self.numberMintedPerPlay[playID]!)
            }
        }

        pub fun retireAll() {
            for play in self.plays {
                self.retirePlay(playID: play)
            }
        }

        pub fun lock() {
            if !self.locked {
                self.locked = true
                emit SetLocked(setID: self.setID)
            }
        }

        pub fun mintTestToken(playID: UInt32): @NFT {
            pre {
                self.retired[playID] != nil: "Cannot mint the testToken: This play doesn't exist"
                !self.retired[playID]!: "Cannot mint the testToken from this play: This play has been retired"
            }
            let numInPlay = self.numberMintedPerPlay[playID]!
            let newTestToken: @NFT <- create NFT(serialNumber: numInPlay + UInt32(1),
                                              playID: playID,
                                              setID: self.setID)

            self.numberMintedPerPlay[playID] = numInPlay + UInt32(1)

            return <-newTestToken
        }

        pub fun batchMintTestToken(playID: UInt32, quantity: UInt64): @Collection {
            let newCollection <- create Collection()

            var i: UInt64 = 0
            while i < quantity {
                newCollection.deposit(token: <-self.mintTestToken(playID: playID))
                i = i + UInt64(1)
            }

            return <-newCollection
        }
    }

    pub struct TestTokenData {
        pub let setID: UInt32
        pub let playID: UInt32
        pub let serialNumber: UInt32

        init(setID: UInt32, playID: UInt32, serialNumber: UInt32) {
            self.setID = setID
            self.playID = playID
            self.serialNumber = serialNumber
        }

    }

    pub resource NFT: NonFungibleToken.INFT {
        pub let id: UInt64
        pub let data: TestTokenData

        init(serialNumber: UInt32, playID: UInt32, setID: UInt32) {
            BatchNFT.totalSupply = BatchNFT.totalSupply + UInt64(1)

            self.id = BatchNFT.totalSupply

            self.data = TestTokenData(setID: setID, playID: playID, serialNumber: serialNumber)

            emit TestTokenMinted(testTokenID: self.id, playID: playID, setID: self.data.setID, serialNumber: self.data.serialNumber)
        }

        destroy() {
            emit TestTokenDestroyed(id: self.id)
        }
    }

    pub resource Admin {
        pub fun createPlay(metadata: {String: String}): UInt32 {
            var newPlay = Play(metadata: metadata)
            let newID = newPlay.playID

            BatchNFT.playDatas[newID] = newPlay

            return newID
        }

        pub fun createSet(name: String) {
            var newSet <- create Set(name: name)

            BatchNFT.sets[newSet.setID] <-! newSet
        }

        pub fun borrowSet(setID: UInt32): &Set {
            pre {
                BatchNFT.sets[setID] != nil: "Cannot borrow Set: The Set doesn't exist"
            }
            return &BatchNFT.sets[setID] as &Set
        }

        pub fun startNewSeries(): UInt32 {
            BatchNFT.currentSeries = BatchNFT.currentSeries + UInt32(1)

            emit NewSeriesStarted(newCurrentSeries: BatchNFT.currentSeries)

            return BatchNFT.currentSeries
        }

        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }
    }

    pub resource interface TestTokenCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowTestToken(id: UInt64): &BatchNFT.NFT? {
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow TestToken reference: The ID of the returned reference is incorrect"
            }
        }
    }

    pub resource Collection: TestTokenCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID)
                ?? panic("Cannot withdraw: TestToken does not exist in the collection")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        pub fun batchWithdraw(ids: [UInt64]): @NonFungibleToken.Collection {
            var batchCollection <- create Collection()

            for id in ids {
                batchCollection.deposit(token: <-self.withdraw(withdrawID: id))
            }
            return <-batchCollection
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @BatchNFT.NFT

            let id = token.id
            let oldToken <- self.ownedNFTs[id] <- token

            if self.owner?.address != nil {
                emit Deposit(id: id, to: self.owner?.address)
            }

            destroy oldToken
        }

        pub fun batchDeposit(tokens: @NonFungibleToken.Collection) {
            let keys = tokens.getIDs()

            for key in keys {
                self.deposit(token: <-tokens.withdraw(withdrawID: key))
            }
            destroy tokens
        }

        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        pub fun borrowTestToken(id: UInt64): &BatchNFT.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &BatchNFT.NFT
            } else {
                return nil
            }
        }
        destroy() {
            destroy self.ownedNFTs
        }
    }

    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <-create BatchNFT.Collection()
    }

    pub fun getAllPlays(): [BatchNFT.Play] {
        return BatchNFT.playDatas.values
    }

    pub fun getPlayMetaData(playID: UInt32): {String: String}? {
        return self.playDatas[playID]?.metadata
    }

    pub fun getPlayMetaDataByField(playID: UInt32, field: String): String? {
        if let play = BatchNFT.playDatas[playID] {
            return play.metadata[field]
        } else {
            return nil
        }
    }

    pub fun getSetName(setID: UInt32): String? {
        return BatchNFT.setDatas[setID]?.name
    }

    pub fun getSetSeries(setID: UInt32): UInt32? {
        return BatchNFT.setDatas[setID]?.series
    }

    pub fun getSetIDsByName(setName: String): [UInt32]? {
        var setIDs: [UInt32] = []

        for setData in BatchNFT.setDatas.values {
            if setName == setData.name {
                setIDs.append(setData.setID)
            }
        }

        if setIDs.length == 0 {
            return nil
        } else {
            return setIDs
        }
    }

    pub fun getPlaysInSet(setID: UInt32): [UInt32]? {
        return BatchNFT.sets[setID]?.plays
    }

    pub fun isEditionRetired(setID: UInt32, playID: UInt32): Bool? {
        if let setToRead <- BatchNFT.sets.remove(key: setID) {
            let retired = setToRead.retired[playID]
            BatchNFT.sets[setID] <-! setToRead
            return retired
        } else {
            return nil
        }
    }

    pub fun isSetLocked(setID: UInt32): Bool? {
        return BatchNFT.sets[setID]?.locked
    }

    pub fun getNumTestTokensInEdition(setID: UInt32, playID: UInt32): UInt32? {
        if let setToRead <- BatchNFT.sets.remove(key: setID) {
            let amount = setToRead.numberMintedPerPlay[playID]
            BatchNFT.sets[setID] <-! setToRead
            return amount
        } else {
            return nil
        }
    }

    init() {
        self.currentSeries = 0
        self.playDatas = {}
        self.setDatas = {}
        self.sets <- {}
        self.nextPlayID = 1
        self.nextSetID = 1
        self.totalSupply = 0

        self.account.save<@Collection>(<- create Collection(), to: /storage/TestTokenCollection)
        self.account.link<&{TestTokenCollectionPublic}>(/public/TestTokenCollection, target: /storage/TestTokenCollection)
        self.account.save<@Admin>(<- create Admin(), to: /storage/BatchNFTAdmin)
        emit ContractInitialized()
    }
}
`, nftAddress.Hex())
	}
	deployContract(b, tctx, executeBlocks, a, pk, "BatchNFT", batchNFTContract(nftAddress))
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
