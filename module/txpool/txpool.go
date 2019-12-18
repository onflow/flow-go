package txpool

import "github.com/dapperlabs/flow-go/model/flow"

// TODO going to rework and replace this with Merkle-tree based impl
type Pool struct {
	transactions []*flow.Transaction
}

func New() *Pool {
	return &Pool{
		transactions: make([]*flow.Transaction, 0),
	}
}

func (p *Pool) Len() int {
	return len(p.transactions)
}

func (p *Pool) Add(tx *flow.Transaction) {
	p.transactions = append(p.transactions, tx)
}
