package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/model/flow"
	"math/rand"
	"strings"
)

type TemplateName string

type TransactionBodyTemplate interface {
	GenerateTransactionBody(
		bodyIndex uint64,
		context *TransactionGenerationContext,
		controllers TransactionLengthControllers,
	) (string, func(executionTime uint64), error)
	Name() TemplateName
	InitialLoopLength() uint64
}

type TransactionTemplate interface {
	GenerateTransaction(*TransactionGenerationContext, TransactionLengthControllers) (GeneratedTransaction, error)
	Name() TemplateName
}

func templateTx(bodySections ...string) string {
	// concatenate all body sections
	body := ""
	for _, section := range bodySections {
		body += section
	}

	return fmt.Sprintf(`
			import EVM from 0xEVM
			import FungibleToken from 0xFUNGIBLETOKEN
			import FlowToken from 0xFLOWTOKEN
			import TestContract from 0xTESTCONTRACT

			transaction(){
				prepare(signer: AuthAccount) {
					%s
				}
			}`, body)
}

func loopTemplateTx(loopNumber uint64, rep uint64, body string) string {
	loopedBody := fmt.Sprintf(`
				var ITERATION = 0
				while ITERATION < %d {
					ITERATION = ITERATION + 1
					%s
				}`, rep, body)

	iteratorName := fmt.Sprintf("II%d", loopNumber)

	return strings.ReplaceAll(loopedBody, "ITERATION", iteratorName)
}

type SimpleTxType struct {
	body string
	name string

	initialLoopLength uint64
}

func (s *SimpleTxType) InitialLoopLength() uint64 {
	return s.initialLoopLength
}

func (s *SimpleTxType) GenerateTransactionBody(
	bodyIndex uint64,
	context *TransactionGenerationContext,
	controllers TransactionLengthControllers,
) (string, func(executionTime uint64), error) {
	maxLoopLength := controllers.GetLoopLength(s.Name())
	loopLength := rand.Uint64()%maxLoopLength + 1
	script := loopTemplateTx(bodyIndex, loopLength, s.body)

	return script,
		func(executionTime uint64) {
			controllers.AdjustParameterRange(s.Name(), loopLength, executionTime)
		},
		nil
}

func (s *SimpleTxType) GenerateTransaction(
	context *TransactionGenerationContext,
	controllers TransactionLengthControllers,
) (GeneratedTransaction, error) {
	script, callback, err := s.GenerateTransactionBody(0, context, controllers)
	if err != nil {
		return GeneratedTransaction{}, err
	}

	script = templateTx(script)
	script = context.ReplaceAddresses(script)

	tx := flow.NewTransactionBody().SetGasLimit(10_000_000).SetScript([]byte(script))

	return GeneratedTransaction{
		Transaction:              tx,
		Name:                     s.Name(),
		AdjustParametersCallback: callback,
	}, nil
}

func (s *SimpleTxType) Name() TemplateName {
	return TemplateName(s.name)
}

var _ TransactionTemplate = &SimpleTxType{}
var _ TransactionBodyTemplate = &SimpleTxType{}

type evmTxType struct {
	generateEVMTX func(index uint64, nonce uint64) *types.Transaction
	name          string

	initialLoopLength uint64
}

func (e *evmTxType) GenerateTransactionBody(
	bodyIndex uint64,
	context *TransactionGenerationContext,
	controllers TransactionLengthControllers,
) (string, func(executionTime uint64), error) {

	maxLoopLength := controllers.GetLoopLength(e.Name())
	loopLength := rand.Uint64()%maxLoopLength + 1

	coinbaseName := fmt.Sprintf("feeAcc%d", bodyIndex)

	body := `
	let COINBASE <- EVM.createBridgedAccount()
	`

	for i := uint64(0); i < loopLength; i++ {

		evmTx := e.generateEVMTX(i, context.GetETHAddressNonce())

		signed, err := types.SignTx(evmTx, emulator.GetDefaultSigner(), context.EthAddressPrivateKey)
		if err != nil {
			return "", nil, err
		}

		var encoded bytes.Buffer
		err = signed.EncodeRLP(&encoded)
		if err != nil {
			return "", nil, err
		}

		body += fmt.Sprintf(`
			EVM.run(tx: "%s".decodeHex(), coinbase: COINBASE.address())
			`, hex.EncodeToString(encoded.Bytes()))
	}

	body += `
destroy COINBASE
`

	body = strings.ReplaceAll(body, "COINBASE", coinbaseName)

	return body, func(executionTime uint64) {
		controllers.AdjustParameterRange(e.Name(), loopLength, executionTime)
	}, nil
}

func (e *evmTxType) InitialLoopLength() uint64 {
	return e.initialLoopLength
}

func (e *evmTxType) GenerateTransaction(context *TransactionGenerationContext, controllers TransactionLengthControllers) (GeneratedTransaction, error) {
	script, callback, err := e.GenerateTransactionBody(0, context, controllers)
	if err != nil {
		return GeneratedTransaction{}, err
	}

	script = templateTx(script)
	script = context.ReplaceAddresses(script)

	tx := flow.NewTransactionBody().SetGasLimit(10_000_000).SetScript([]byte(script))

	return GeneratedTransaction{
		Transaction:              tx,
		Name:                     e.Name(),
		AdjustParametersCallback: callback,
	}, nil
}

func (e *evmTxType) Name() TemplateName {
	return TemplateName(e.name)
}

var _ TransactionTemplate = &evmTxType{}
var _ TransactionBodyTemplate = &evmTxType{}

type MixedTxType struct {
	bodyTemplates []TransactionBodyTemplate
}

func (m *MixedTxType) GenerateTransaction(
	context *TransactionGenerationContext,
	controller TransactionLengthControllers,
) (GeneratedTransaction, error) {

	bodies := make([]string, len(m.bodyTemplates))
	for i, tt := range m.bodyTemplates {
		body, _, err := tt.GenerateTransactionBody(uint64(i), context, controller)
		if err != nil {
			return GeneratedTransaction{}, err
		}
		bodies[i] = body
	}

	script := templateTx(bodies...)
	script = context.ReplaceAddresses(script)
	tx := flow.NewTransactionBody().SetGasLimit(1_000_000).SetScript([]byte(script))

	return GeneratedTransaction{
		Transaction:              tx,
		Name:                     m.Name(),
		AdjustParametersCallback: func(executionTime uint64) {},
	}, nil
}

func (m *MixedTxType) Name() TemplateName {
	// concatenate the names of all simple types
	names := make([]string, len(m.bodyTemplates))
	for i, t := range m.bodyTemplates {
		names[i] = string(t.Name())
	}
	return TemplateName(strings.Join(names, " + "))
}

var _ TransactionTemplate = &MixedTxType{}
