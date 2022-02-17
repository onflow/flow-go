package request

import (
	"io"

	"github.com/onflow/flow-go/model/flow"
)

type CreateTransaction struct {
	Transaction flow.TransactionBody
}

func (c *CreateTransaction) Build(r *Request) error {
	return c.Parse(r.Body, r.Chain)
}

func (c *CreateTransaction) Parse(rawTransaction io.Reader, chain flow.Chain) error {
	var tx Transaction
	err := tx.Parse(rawTransaction, chain)
	if err != nil {
		return err
	}

	c.Transaction = tx.Flow()
	return nil
}
