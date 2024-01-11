package main

import (
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	evmTypes "github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
	"math/big"
	"math/rand"
)

type GeneratedTransaction struct {
	Transaction              *flow.TransactionBody
	Name                     TemplateName
	AdjustParametersCallback func(executionTime uint64)
}

type TransactionGenerator struct {
	templates []TransactionTemplate

	controllers TransactionLengthControllers
}

func NewTransactionGenerator(templates []TransactionTemplate) *TransactionGenerator {
	return &TransactionGenerator{
		templates:   templates,
		controllers: NewTransactionLengthControllers(templates),
	}
}

func (p *TransactionGenerator) Generate(context *TransactionGenerationContext) (GeneratedTransaction, error) {
	template := p.chooseTemplate()

	return template.GenerateTransaction(context, p.controllers)
}

func (p *TransactionGenerator) chooseTemplate() TransactionTemplate {
	// 2 out of 3 transactions should be of type `SimpleTxType`
	if rand.Intn(3) == 0 {
		return p.templates[rand.Intn(len(p.templates))]
	}

	// otherwise, return a `MixedTxType` with 2 different SimpleTxTypes
	a := rand.Intn(len(p.templates))
	b := a
	for b == a {
		b = rand.Intn(len(p.templates))
	}
	return &MixedTxType{
		bodyTemplates: []TransactionBodyTemplate{
			p.templates[a].(TransactionBodyTemplate),
			p.templates[b].(TransactionBodyTemplate),
		},
	}
}

var Generator = NewTransactionGenerator(
	[]TransactionTemplate{
		&SimpleTxType{
			initialLoopLength: 586823,
			body:              "",
			name:              "reference tx",
		},
		&SimpleTxType{
			initialLoopLength: 268918,
			body:              "ITERATION.toString()",
			name:              "convert int to string",
		},
		&SimpleTxType{
			initialLoopLength: 137209,
			body:              `"x".concat(ITERATION.toString())`,
			name:              "convert int to string and concatenate it",
		},
		&SimpleTxType{
			initialLoopLength: 366892,
			body:              `signer.address`,
			name:              "get signer address",
		},
		&SimpleTxType{
			initialLoopLength: 131482,
			body:              `getAccount(signer.address)`,
			name:              "get public account",
		},
		&SimpleTxType{
			initialLoopLength: 1089,
			body:              `getAccount(signer.address).balance`,
			name:              "get account and get balance",
		},
		&SimpleTxType{
			initialLoopLength: 1317,
			body:              `getAccount(signer.address).availableBalance`,
			name:              "get account and get available balance",
		},
		&SimpleTxType{
			initialLoopLength: 32273,
			body:              `getAccount(signer.address).storageUsed`,
			name:              "get account and get storage used",
		},
		&SimpleTxType{
			initialLoopLength: 1577,
			body:              `getAccount(signer.address).storageCapacity`,
			name:              "get account and get storage capacity",
		},
		&SimpleTxType{
			initialLoopLength: 72169,
			body:              `let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)!`,
			name:              "get signer vault",
		},
		&SimpleTxType{
			initialLoopLength: 32369,
			body: `let receiverRef = getAccount(signer.address)
				.getCapability(/public/flowTokenReceiver)
				.borrow<&{FungibleToken.Receiver}>()!`,
			name: "get signer receiver",
		},
		&SimpleTxType{
			initialLoopLength: 4394,
			body: `let receiverRef =  getAccount(signer.address)
				.getCapability(/public/flowTokenReceiver)
				.borrow<&{FungibleToken.Receiver}>()!
				let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)!
				receiverRef.deposit(from: <-vaultRef.withdraw(amount: 0.00001))`,
			name: "transfer tokens",
		},
		&SimpleTxType{
			initialLoopLength: 93077,
			body: `signer.load<String>(from: /storage/testpath)
				signer.save("", to: /storage/testpath)`,
			name: "load and save empty string on signers address",
		},
		&SimpleTxType{
			initialLoopLength: 95005,
			body: `signer.load<String>(from: /storage/testpath)
				signer.save("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", to: /storage/testpath)`,
			name: "load and save long string on signers address",
		},
		&SimpleTxType{
			initialLoopLength: 338,
			body:              `let acct = AuthAccount(payer: signer)`,
			name:              "create new account",
		},
		&SimpleTxType{
			initialLoopLength: 280,
			body: `
				let acct = AuthAccount(payer: signer)
				acct.contracts.add(name: "EmptyContract", code: "61636365737328616c6c2920636f6e747261637420456d707479436f6e7472616374207b7d".decodeHex())
			`,
			name: "create new account and deploy contract",
		},
		&SimpleTxType{
			initialLoopLength: 118866,
			body:              `TestContract.empty()`,
			name:              "call empty contract function",
		},
		&SimpleTxType{
			initialLoopLength: 53391,
			body:              `TestContract.emit()`,
			name:              "emit event",
		},
		&SimpleTxType{
			initialLoopLength: 14188,
			body: `let strings = signer.borrow<&[String]>(from: /storage/test)!
				var j = 0
				var lenSum = 0
				while (j < strings.length) {
				  	lenSum = lenSum + strings[j].length
					j = j + 1
				}`,
			name: "borrow array from storage",
		},
		&SimpleTxType{
			initialLoopLength: 19225,
			body: `let strings = signer.copy<[String]>(from: /storage/test)!
				var j = 0
				var lenSum = 0
				while (j < strings.length) {
				  	lenSum = lenSum + strings[j].length
					j = j + 1
				}`,
			name: "copy array from storage",
		},
		&SimpleTxType{
			initialLoopLength: 1002,
			body: `let strings = signer.copy<[String]>(from: /storage/test)!
				var j = 0
				var lenSum = 0
				while (j < strings.length) {
				  	lenSum = lenSum + strings[j].length
					j = j + 1
				}
				signer.load<[String]>(from: /storage/test2)
				signer.save(strings, to: /storage/test2)
				`,
			name: "copy array from storage and save a duplicate",
		},
		&SimpleTxType{
			initialLoopLength: 5147,
			body:              `signer.addPublicKey("f847b84000fb479cb398ab7e31d6f048c12ec5b5b679052589280cacde421af823f93fe927dfc3d1e371b172f97ceeac1bc235f60654184c83f4ea70dd3b7785ffb3c73802038203e8".decodeHex())`,
			name:              "add key to account",
		},
		&SimpleTxType{
			initialLoopLength: 2956,
			body: `
				signer.addPublicKey("f847b84000fb479cb398ab7e31d6f048c12ec5b5b679052589280cacde421af823f93fe927dfc3d1e371b172f97ceeac1bc235f60654184c83f4ea70dd3b7785ffb3c73802038203e8".decodeHex())
				signer.removePublicKey(1)
			`,
			name: "add and remove key to/from account",
		},
		&SimpleTxType{
			initialLoopLength: 9519,
			body:              `TestContract.mintNFT()`,
			name:              "mint NFT",
		},
		&SimpleTxType{
			initialLoopLength: 9519,
			body:              `"f847b84000fb479cb398ab7e31d6f048c12ec5b5b679052589280cacde421af823f93fe927dfc3d1e371b172f97ceeac1bc235f60654184c83f4ea70dd3b7785ffb3c73802038203e8".decodeHex()`,
			name:              "decode hex",
		},
		&SimpleTxType{
			initialLoopLength: 500,
			body: `let feeAcc <- EVM.createBridgedAccount()
					destroy feeAcc`,
			name: "create and destroy EVM bridged account",
		},
		&evmTxType{
			initialLoopLength: 250,
			generateEVMTX: func(index uint64, nonce uint64) *types.Transaction {
				to := gethcommon.HexToAddress("")
				gasPrice := big.NewInt(0)
				return types.NewTx(&types.LegacyTx{Nonce: nonce, To: &to, Value: evmTypes.OneFlowInAttoFlow, Gas: params.TxGas, GasPrice: gasPrice, Data: nil})
			},
			name: "transfer tokens to EVM address",
		},
	},
)
