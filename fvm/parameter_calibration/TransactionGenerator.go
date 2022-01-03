package main

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"math/rand"
	"strings"
)

// get a random transaction type from a pool of transaction types
// generate a transaction of that type with one parameter randomly selected from the range
// use transaction execution time to tweak the parameter range

type TransactionTypeContext struct {
	AddressReplacements map[string]string
}

func (ctx TransactionTypeContext) ReplaceAddresses(tx string) string {
	for key, value := range ctx.AddressReplacements {
		tx = strings.ReplaceAll(tx, key, value)
	}
	return tx
}

type GeneratedTransaction struct {
	Transaction *flow.TransactionBody
	Type        TransactionType
	Parameter   uint64
}

func (t GeneratedTransaction) AdjustParameterRange(u uint64) {
	t.Type.AdjustParameterRange(t.Parameter, u)
}

type TransactionType interface {
	GenerateTransaction(TransactionTypeContext) (GeneratedTransaction, error)
	AdjustParameterRange(parameter uint64, executionTime uint64)
	Name() string
}

type TransactionTypePool struct {
	Pool []TransactionType
}

func (p *TransactionTypePool) GetRandomTransactionType() TransactionType {
	return p.Pool[rand.Intn(len(p.Pool))]
}

func templateTx(rep uint64, body string) string {
	return fmt.Sprintf(`
			import FungibleToken from 0xFUNGIBLETOKEN
			import FlowToken from 0xFLOWTOKEN
			import TestContract from 0xTESTCONTRACT

			transaction(){
				prepare(signer: AuthAccount){
					var i = 0
					while i < %d {
						i = i + 1
						%s
					}
				}
			}`, rep, body)
}

type SimpleTxType struct {
	paramMin uint64
	paramMax uint64
	body     string
	name     string
}

func (s *SimpleTxType) GenerateTransaction(context TransactionTypeContext) (GeneratedTransaction, error) {
	parameter := rand.Uint64() % (s.paramMax - s.paramMin)
	parameter += s.paramMin
	script := templateTx(parameter, s.body)
	script = context.ReplaceAddresses(script)
	tx := flow.NewTransactionBody().SetGasLimit(1_000_000).SetScript([]byte(script))

	return GeneratedTransaction{
		Transaction: tx,
		Type:        s,
		Parameter:   parameter,
	}, nil
}

func (s *SimpleTxType) Name() string {
	return s.name
}

func (s *SimpleTxType) AdjustParameterRange(parameter uint64, executionTime uint64) {
	if executionTime < 10 {
		s.paramMin *= 2
		s.paramMax *= 2
		fmt.Println("Adjusting parameter range for", s.name, "to", s.paramMin, "to", s.paramMax)
	}
	if executionTime > 1000 {
		s.paramMin /= 2
		if s.paramMin < 1 {
			s.paramMin = 1
		}
		s.paramMax /= 2
		if s.paramMax < 1 {
			s.paramMax = 1
		}
		fmt.Println("Adjusting parameter range for", s.name, "to", s.paramMin, "to", s.paramMax)
	}
}

var _ TransactionType = &SimpleTxType{}

var Pool = TransactionTypePool{
	Pool: []TransactionType{
		&SimpleTxType{
			paramMin: 50,
			paramMax: 1000,
			body:     "",
			name:     "reference tx",
		},
		&SimpleTxType{
			paramMin: 50,
			paramMax: 1000,
			body:     "i.toString()",
			name:     "convert int to string",
		},
		&SimpleTxType{
			paramMin: 50,
			paramMax: 1000,
			body:     `"x".concat(i.toString())`,
			name:     "convert int to string and concatenate it",
		},
		&SimpleTxType{
			paramMin: 50,
			paramMax: 1000,
			body:     `signer.address`,
			name:     "get signer address",
		},
		&SimpleTxType{
			paramMin: 50,
			paramMax: 1000,
			body:     `getAccount(signer.address)`,
			name:     "get public account",
		},
		&SimpleTxType{
			paramMin: 25,
			paramMax: 500,
			body:     `getAccount(signer.address).balance`,
			name:     "get account and get balance",
		},
		&SimpleTxType{
			paramMin: 25,
			paramMax: 500,
			body:     `getAccount(signer.address).availableBalance`,
			name:     "get account and get available balance",
		},
		&SimpleTxType{
			paramMin: 100,
			paramMax: 2000,
			body:     `getAccount(signer.address).storageUsed`,
			name:     "get account and get storage used",
		},
		&SimpleTxType{
			paramMin: 50,
			paramMax: 1000,
			body:     `getAccount(signer.address).storageCapacity`,
			name:     "get account and get storage capacity",
		},
		&SimpleTxType{
			paramMin: 50,
			paramMax: 1000,
			body:     `getAccount(signer.address).storageCapacity`,
			name:     "get account and get storage capacity",
		},
		&SimpleTxType{
			paramMin: 50,
			paramMax: 1000,
			body:     `getAccount(signer.address).storageCapacity`,
			name:     "get account and get storage capacity",
		},
		&SimpleTxType{
			paramMin: 50,
			paramMax: 1000,
			body:     `getAccount(signer.address).storageCapacity`,
			name:     "get account and get storage capacity",
		},
		&SimpleTxType{
			paramMin: 50,
			paramMax: 1000,
			body:     `let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)!`,
			name:     "get signer vault",
		},
		&SimpleTxType{
			paramMin: 50,
			paramMax: 1000,
			body: `let receiverRef =  getAccount(signer.address)
				.getCapability(/public/flowTokenReceiver)
				.borrow<&{FungibleToken.Receiver}>()!`,
			name: "get signer receiver",
		},
		&SimpleTxType{
			paramMin: 50,
			paramMax: 1000,
			body: `let receiverRef =  getAccount(signer.address)
				.getCapability(/public/flowTokenReceiver)
				.borrow<&{FungibleToken.Receiver}>()!
				let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)!
				receiverRef.deposit(from: <-vaultRef.withdraw(amount: 0.00001))`,
			name: "transfer tokens",
		},
		&SimpleTxType{
			paramMin: 50,
			paramMax: 1000,
			body: `let receiverRef =  getAccount(signer.address)
				.getCapability(/public/flowTokenReceiver)
				.borrow<&{FungibleToken.Receiver}>()!`,
			name: "get signer receiver",
		},
		&SimpleTxType{
			paramMin: 50,
			paramMax: 1000,
			body: `signer.load<String>(from: /storage/testpath)
				signer.save("", to: /storage/testpath)`,
			name: "load and save empty string on signers address",
		},
		&SimpleTxType{
			paramMin: 50,
			paramMax: 1000,
			body: `signer.load<String>(from: /storage/testpath)
				signer.save("%s", to: /storage/testpath)`,
			name: "load and save long string on signers address",
		},
		&SimpleTxType{
			paramMin: 5,
			paramMax: 100,
			body:     `let acct = AuthAccount(payer: signer)`,
			name:     "create new account",
		},
		&SimpleTxType{
			paramMin: 50,
			paramMax: 1000,
			body:     `TestContract.empty()`,
			name:     "call empty contract function",
		},
		&SimpleTxType{
			paramMin: 50,
			paramMax: 1000,
			body:     `TestContract.emit()`,
			name:     "emit event",
		},
		&SimpleTxType{
			paramMin: 500,
			paramMax: 10000,
			body: `let strings = signer.borrow<&[String]>(from: /storage/test)!
				var j = 0
				var lenSum = 0
				while (j < strings.length) {
				  	lenSum = lenSum + strings[j].length
					j = j + 1
				}`,
			name: "call empty contract function",
		},
		&SimpleTxType{
			paramMin: 50,
			paramMax: 1000,
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
			paramMin: 50,
			paramMax: 1000,
			body:     `signer.addPublicKey("f847b84000fb479cb398ab7e31d6f048c12ec5b5b679052589280cacde421af823f93fe927dfc3d1e371b172f97ceeac1bc235f60654184c83f4ea70dd3b7785ffb3c73802038203e8".decodeHex())`,
			name:     "add key to account",
		},
		&SimpleTxType{
			paramMin: 50,
			paramMax: 1000,
			body: `
				signer.addPublicKey("f847b84000fb479cb398ab7e31d6f048c12ec5b5b679052589280cacde421af823f93fe927dfc3d1e371b172f97ceeac1bc235f60654184c83f4ea70dd3b7785ffb3c73802038203e8".decodeHex())
				signer.removePublicKey(1)
			`,
			name: "add and remove key to/from account",
		},
	},
}
