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
	//return p.Pool[rand.Intn(len(p.Pool))]
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
		var loopLength uint64
		if tt.slopePoints == 0 {
			loopLength = tt.paramMax/uint64(len(m.simpleTypes)) + 1
		} else {
			loopLength = rand.Uint64()%(tt.paramMax/uint64(len(m.simpleTypes))) + 1 //tt.paramMax/uint64(len(m.simpleTypes))/5 + 1 //
		}
		bodies[i] = loopTemplateTx(loopLength, tt.body)
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

	var loopLength uint64
	if s.slopePoints == 0 {
		loopLength = s.paramMax + 1 //s.paramMax/5 + 1 //
	} else {
		loopLength = rand.Uint64()%s.paramMax + 1 // s.paramMax/5 + 1 //
	}

	script := simpleTemplateTx(loopLength, s.body)
	script = context.ReplaceAddresses(script)
	tx := flow.NewTransactionBody().SetGasLimit(10_000_000).SetScript([]byte(script))

	return GeneratedTransaction{
		Transaction: tx,
		Type:        s,
		Parameter:   loopLength,
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
			paramMax: 586823,
			body:     "",
			name:     "reference tx",
		},
		&SimpleTxType{
			paramMax: 268918,
			body:     "i.toString()",
			name:     "convert int to string",
		},
		&SimpleTxType{
			paramMax: 137209,
			body:     `"x".concat(i.toString())`,
			name:     "convert int to string and concatenate it",
		},
		&SimpleTxType{
			paramMax: 366892,
			body:     `signer.address`,
			name:     "get signer address",
		},
		&SimpleTxType{
			paramMax: 131482,
			body:     `getAccount(signer.address)`,
			name:     "get public account",
		},
		&SimpleTxType{
			paramMax: 1089,
			body:     `getAccount(signer.address).balance`,
			name:     "get account and get balance",
		},
		&SimpleTxType{
			paramMax: 1317,
			body:     `getAccount(signer.address).availableBalance`,
			name:     "get account and get available balance",
		},
		&SimpleTxType{
			paramMax: 32273,
			body:     `getAccount(signer.address).storageUsed`,
			name:     "get account and get storage used",
		},
		&SimpleTxType{
			paramMax: 1577,
			body:     `getAccount(signer.address).storageCapacity`,
			name:     "get account and get storage capacity",
		},
		&SimpleTxType{
			paramMax: 72169,
			body:     `let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)!`,
			name:     "get signer vault",
		},
		&SimpleTxType{
			paramMax: 32369,
			body: `let receiverRef = getAccount(signer.address)
				.getCapability(/public/flowTokenReceiver)
				.borrow<&{FungibleToken.Receiver}>()!`,
			name: "get signer receiver",
		},
		&SimpleTxType{
			paramMax: 4394,
			body: `let receiverRef =  getAccount(signer.address)
				.getCapability(/public/flowTokenReceiver)
				.borrow<&{FungibleToken.Receiver}>()!
				let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)!
				receiverRef.deposit(from: <-vaultRef.withdraw(amount: 0.00001))`,
			name: "transfer tokens",
		},
		&SimpleTxType{
			paramMax: 93077,
			body: `signer.load<String>(from: /storage/testpath)
				signer.save("", to: /storage/testpath)`,
			name: "load and save empty string on signers address",
		},
		&SimpleTxType{
			paramMax: 95005,
			body: `signer.load<String>(from: /storage/testpath)
				signer.save("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", to: /storage/testpath)`,
			name: "load and save long string on signers address",
		},
		&SimpleTxType{
			paramMax: 338,
			body:     `let acct = AuthAccount(payer: signer)`,
			name:     "create new account",
		},
		&SimpleTxType{
			paramMax: 280,
			body: `
				let acct = AuthAccount(payer: signer)
				acct.contracts.add(name: "EmptyContract", code: "61636365737328616c6c2920636f6e747261637420456d707479436f6e7472616374207b7d".decodeHex())
			`,
			name: "create new account and deploy contract",
		},
		&SimpleTxType{
			paramMax: 118866,
			body:     `TestContract.empty()`,
			name:     "call empty contract function",
		},
		&SimpleTxType{
			paramMax: 53391,
			body:     `TestContract.emit()`,
			name:     "emit event",
		},
		&SimpleTxType{
			paramMax: 14188,
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
			paramMax: 19225,
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
			paramMax: 1002,
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
			paramMax: 5147,
			body:     `signer.addPublicKey("f847b84000fb479cb398ab7e31d6f048c12ec5b5b679052589280cacde421af823f93fe927dfc3d1e371b172f97ceeac1bc235f60654184c83f4ea70dd3b7785ffb3c73802038203e8".decodeHex())`,
			name:     "add key to account",
		},
		&SimpleTxType{
			paramMax: 2956,
			body: `
				signer.addPublicKey("f847b84000fb479cb398ab7e31d6f048c12ec5b5b679052589280cacde421af823f93fe927dfc3d1e371b172f97ceeac1bc235f60654184c83f4ea70dd3b7785ffb3c73802038203e8".decodeHex())
				signer.removePublicKey(1)
			`,
			name: "add and remove key to/from account",
		},
		&SimpleTxType{
			paramMax: 9519,
			body:     `TestContract.mintNFT()`,
			name:     "mint NFT",
		},
	},
}

/*
reference tx 0.0008965043725548607
convert int to string 0.0019021032132351444
convert int to string and concatenate it 0.003644069664727692
get signer address 0.0013627983083240906
get public account 0.003802797860297355
get account and get balance 0.4589855321006071
get account and get available balance 0.3795063117766648
get account and get storage used 0.015492741660975237
get account and get storage capacity 0.3168937469440481
get signer vault 0.006928146420386147
get signer receiver 0.015446848725147693
transfer tokens 0.11378142139922551
load and save empty string on signers address 0.005371884335269915
load and save long string on signers address 0.005262836957112487
create new account 1.4778613109038294
create new account and deploy contract 1.7798558435076237
call empty contract function 0.0042064064196769245
emit event 0.009364837959575923
borrow array from storage 0.03523907668395154
copy array from storage 0.026006660839544348
copy array from storage and save a duplicate 0.4989533979727946
add key to account 0.09713563390921647
add and remove key to/from account 0.16911184458028658
mint NFT 0.05252269085227618

*/
