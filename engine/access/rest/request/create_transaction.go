package request

import (
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/onflow/flow-go/model/flow"
	"io"
)

type CreateTransaction struct {
	Transaction flow.TransactionBody
}

func (c *CreateTransaction) Build(r *rest.Request) error {

}

func (c *CreateTransaction) Parse(rawTransaction io.Reader) error {

	var tx Transaction
	err := tx.Parse(rawTransaction)
	if err != nil {
		return err
	}

	c.Transaction = tx.Flow()
}
