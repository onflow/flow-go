package request

const ExpandsTransactions = "transactions"

type GetCollection struct {
	GetByIDRequest
	ExpandsTransactions bool
}

func (g *GetCollection) Build(r *Request) error {
	err := g.GetByIDRequest.Build(r)
	g.ExpandsTransactions = r.Expands(ExpandsTransactions)

	return err
}
