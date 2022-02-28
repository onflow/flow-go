package main

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"

	"github.com/onflow/flow-go/model/flow"
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
	if st, ok := t.Type.(*SimpleTxType); ok {
		st.AdjustParameterRange(t.Parameter, u)
	}
}

type TransactionType interface {
	GenerateTransaction(TransactionTypeContext) (GeneratedTransaction, error)
	Name() string
}

type TransactionTypePool struct {
	Pool []TransactionType
}

func (p *TransactionTypePool) GetRandomTransactionType() TransactionType {
	// 2 out of 3 transactions should be of type `SimpleTxType`
	if rand.Intn(3) == 0 {
		return p.Pool[rand.Intn(len(p.Pool))]
	}
	// otherwise, return a `MixedTxType` with 2 different SimpleTxTypes
	a := rand.Intn(len(p.Pool))
	b := a
	for b == a {
		b = rand.Intn(len(p.Pool))
	}
	return &MixedTxType{
		simpleTypes: []*SimpleTxType{
			p.Pool[a].(*SimpleTxType),
			p.Pool[b].(*SimpleTxType),
		},
	}
}

func simpleTemplateTx(rep uint64, body string) string {
	return templateTx(loopTemplateTx(rep, body))
}

func templateTx(bodySections ...string) string {
	// concatenate all body sections
	body := ""
	for _, section := range bodySections {
		body += section
	}

	return fmt.Sprintf(`
			import FungibleToken from 0xFUNGIBLETOKEN
			import FlowToken from 0xFLOWTOKEN
			import TestContract from 0xTESTCONTRACT

			transaction(){
				prepare(signer: AuthAccount){
					var %s
				}
			}`, body)
}

func loopTemplateTx(rep uint64, body string) string {
	return fmt.Sprintf(
		`i = 0
	while i < %d {
		i = i + 1
		%s
	}
	`, rep, body)
}

type MixedTxType struct {
	simpleTypes []*SimpleTxType
}

func (m *MixedTxType) GenerateTransaction(context TransactionTypeContext) (GeneratedTransaction, error) {
	// lock all simple types to prevent concurrent access
	for _, tt := range m.simpleTypes {
		tt.mu.Lock()
	}
	defer func() {
		for _, tt := range m.simpleTypes {
			tt.mu.Unlock()
		}
	}()

	bodies := make([]string, len(m.simpleTypes))
	for i, tt := range m.simpleTypes {
		parameter := rand.Uint64()%(tt.paramMax/uint64(len(m.simpleTypes))) + 1
		if tt.slopePoints == 0 {
			parameter = tt.paramMax/uint64(len(m.simpleTypes)) + 1
		}
		bodies[i] = loopTemplateTx(parameter, tt.body)
	}

	script := templateTx(bodies...)
	script = context.ReplaceAddresses(script)
	tx := flow.NewTransactionBody().SetGasLimit(1_000_000).SetScript([]byte(script))

	return GeneratedTransaction{
		Transaction: tx,
		Type:        m,
		Parameter:   0,
	}, nil

}

func (m *MixedTxType) Name() string {
	// concatenate the names of all simple types
	names := make([]string, len(m.simpleTypes))
	for i, t := range m.simpleTypes {
		names[i] = t.Name()
	}
	return strings.Join(names, " + ")
}

var _ TransactionType = &MixedTxType{}

type SimpleTxType struct {
	paramMax uint64
	body     string
	name     string

	mu          sync.Mutex
	slopePoints uint64
	slope       float64
}

var desiredMaxTime float64 = 500 // in milliseconds

func (s *SimpleTxType) GenerateTransaction(context TransactionTypeContext) (GeneratedTransaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	parameter := rand.Uint64()%s.paramMax + 1
	if s.slopePoints == 0 {
		parameter = s.paramMax + 1
	}
	script := simpleTemplateTx(parameter, s.body)
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

// AdjustParameterRange adjusts the parameter range of the transaction type so that the generated transactions take up to desiredMaxTime
func (s *SimpleTxType) AdjustParameterRange(parameter uint64, executionTime uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	prevParamMax := s.paramMax

	s.slope = (float64(s.slopePoints)*s.slope + (float64(executionTime) / float64(parameter))) / float64(s.slopePoints+1)
	s.slopePoints++

	s.paramMax = uint64(desiredMaxTime / s.slope)
	if s.paramMax < 1 {
		s.paramMax = 1
	}
	if (float64(s.paramMax)-float64(prevParamMax))/float64(prevParamMax) > 2.0 {
		s.paramMax = prevParamMax * 2
	}
}

func (s *SimpleTxType) GetSlope() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.slope
}

var _ TransactionType = &SimpleTxType{}

var Pool = TransactionTypePool{
	Pool: []TransactionType{
		&SimpleTxType{
			paramMax: 1000,
			body:     "",
			name:     "reference tx",
		},
		&SimpleTxType{
			paramMax: 1000,
			body:     "i.toString()",
			name:     "convert int to string",
		},
		&SimpleTxType{
			paramMax: 1000,
			body:     `"x".concat(i.toString())`,
			name:     "convert int to string and concatenate it",
		},
		&SimpleTxType{
			paramMax: 1000,
			body:     `signer.address`,
			name:     "get signer address",
		},
		&SimpleTxType{
			paramMax: 1000,
			body:     `getAccount(signer.address)`,
			name:     "get public account",
		},
		&SimpleTxType{
			paramMax: 500,
			body:     `getAccount(signer.address).balance`,
			name:     "get account and get balance",
		},
		&SimpleTxType{
			paramMax: 500,
			body:     `getAccount(signer.address).availableBalance`,
			name:     "get account and get available balance",
		},
		&SimpleTxType{
			paramMax: 2000,
			body:     `getAccount(signer.address).storageUsed`,
			name:     "get account and get storage used",
		},
		&SimpleTxType{
			paramMax: 1000,
			body:     `getAccount(signer.address).storageCapacity`,
			name:     "get account and get storage capacity",
		},
		&SimpleTxType{
			paramMax: 1000,
			body:     `let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)!`,
			name:     "get signer vault",
		},
		&SimpleTxType{
			paramMax: 1000,
			body: `let receiverRef = getAccount(signer.address)
				.getCapability(/public/flowTokenReceiver)
				.borrow<&{FungibleToken.Receiver}>()!`,
			name: "get signer receiver",
		},
		&SimpleTxType{
			paramMax: 1000,
			body: `let receiverRef =  getAccount(signer.address)
				.getCapability(/public/flowTokenReceiver)
				.borrow<&{FungibleToken.Receiver}>()!
				let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)!
				receiverRef.deposit(from: <-vaultRef.withdraw(amount: 0.00001))`,
			name: "transfer tokens",
		},
		&SimpleTxType{
			paramMax: 1000,
			body: `signer.load<String>(from: /storage/testpath)
				signer.save("", to: /storage/testpath)`,
			name: "load and save empty string on signers address",
		},
		&SimpleTxType{
			paramMax: 1000,
			body: `signer.load<String>(from: /storage/testpath)
				signer.save("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", to: /storage/testpath)`,
			name: "load and save long string on signers address",
		},
		&SimpleTxType{
			paramMax: 100,
			body:     `let acct = AuthAccount(payer: signer)`,
			name:     "create new account",
		},
		&SimpleTxType{
			paramMax: 100,
			body: `
				let acct = AuthAccount(payer: signer)
				acct.contracts.add(name: "EmptyContract", code: "61636365737328616c6c2920636f6e747261637420456d707479436f6e7472616374207b7d".decodeHex())
			`,
			name: "create new account and deploy contract",
		},
		&SimpleTxType{
			paramMax: 1000,
			body:     `TestContract.empty()`,
			name:     "call empty contract function",
		},
		&SimpleTxType{
			paramMax: 1000,
			body:     `TestContract.emit()`,
			name:     "emit event",
		},
		&SimpleTxType{
			paramMax: 1000,
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
			paramMax: 1000,
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
			paramMax: 1000,
			body:     `signer.addPublicKey("f847b84000fb479cb398ab7e31d6f048c12ec5b5b679052589280cacde421af823f93fe927dfc3d1e371b172f97ceeac1bc235f60654184c83f4ea70dd3b7785ffb3c73802038203e8".decodeHex())`,
			name:     "add key to account",
		},
		&SimpleTxType{
			paramMax: 1000,
			body: `
				signer.addPublicKey("f847b84000fb479cb398ab7e31d6f048c12ec5b5b679052589280cacde421af823f93fe927dfc3d1e371b172f97ceeac1bc235f60654184c83f4ea70dd3b7785ffb3c73802038203e8".decodeHex())
				signer.removePublicKey(1)
			`,
			name: "add and remove key to/from account",
		},
		&SimpleTxType{
			paramMax: 100,
			body:     `TestContract.mintNFT()`,
			name:     "mint NFT",
		},
	},
}
