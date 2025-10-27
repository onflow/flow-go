package fixtures

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// TransactionErrorMessage is the default options factory for [flow.TransactionResultErrorMessage] generation.
var TransactionErrorMessage transactionErrorMessageFactory

type transactionErrorMessageFactory struct{}

type TransactionErrorMessageOption func(*TransactionErrorMessageGenerator, *flow.TransactionResultErrorMessage)

// WithTransactionID is an option that sets the transaction ID for the transaction result error message.
func (f transactionErrorMessageFactory) WithTransactionID(transactionID flow.Identifier) TransactionErrorMessageOption {
	return func(g *TransactionErrorMessageGenerator, err *flow.TransactionResultErrorMessage) {
		err.TransactionID = transactionID
	}
}

// WithIndex is an option that sets the index for the transaction result error message.
func (f transactionErrorMessageFactory) WithIndex(index uint32) TransactionErrorMessageOption {
	return func(g *TransactionErrorMessageGenerator, err *flow.TransactionResultErrorMessage) {
		err.Index = index
	}
}

// WithErrorMessage is an option that sets the error message for the transaction result error message.
func (f transactionErrorMessageFactory) WithErrorMessage(errorMessage string) TransactionErrorMessageOption {
	return func(g *TransactionErrorMessageGenerator, err *flow.TransactionResultErrorMessage) {
		err.ErrorMessage = errorMessage
	}
}

// WithExecutorID is an option that sets the executor ID for the transaction result error message.
func (f transactionErrorMessageFactory) WithExecutorID(executorID flow.Identifier) TransactionErrorMessageOption {
	return func(g *TransactionErrorMessageGenerator, err *flow.TransactionResultErrorMessage) {
		err.ExecutorID = executorID
	}
}

type TransactionErrorMessageGenerator struct {
	transactionErrorMessageFactory

	random      *RandomGenerator
	identifiers *IdentifierGenerator
}

func NewTransactionErrorMessageGenerator(
	random *RandomGenerator,
	identifiers *IdentifierGenerator,
) *TransactionErrorMessageGenerator {
	return &TransactionErrorMessageGenerator{
		random:      random,
		identifiers: identifiers,
	}
}

func (g *TransactionErrorMessageGenerator) Fixture(opts ...TransactionErrorMessageOption) flow.TransactionResultErrorMessage {
	transactionID := g.identifiers.Fixture()
	txErrMsg := flow.TransactionResultErrorMessage{
		TransactionID: transactionID,
		Index:         g.random.Uint32InRange(0, 100),
		ErrorMessage:  fmt.Sprintf("transaction error for %s", transactionID),
		ExecutorID:    g.identifiers.Fixture(),
	}

	for _, opt := range opts {
		opt(g, &txErrMsg)
	}

	return txErrMsg
}

func (g *TransactionErrorMessageGenerator) List(n int, opts ...TransactionErrorMessageOption) []flow.TransactionResultErrorMessage {
	list := make([]flow.TransactionResultErrorMessage, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}

func (g *TransactionErrorMessageGenerator) ForTransactionResults(transactionResults []flow.LightTransactionResult) []flow.TransactionResultErrorMessage {
	txErrMsgs := make([]flow.TransactionResultErrorMessage, 0)
	for i, result := range transactionResults {
		if result.Failed {
			txErrMsgs = append(txErrMsgs, g.Fixture(
				g.WithTransactionID(result.TransactionID),
				g.WithIndex(uint32(i)),
				g.WithErrorMessage(fmt.Sprintf("transaction error for %s", result.TransactionID)),
			))
		}
	}
	return txErrMsgs
}
