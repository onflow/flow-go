package request

const resultExpandable = "result"

type GetTransaction struct {
	GetByIDRequest
	ExpandsResult bool
}

func (g *GetTransaction) Build(r *Request) error {
	err := g.GetByIDRequest.Build(r)
	g.ExpandsResult = r.Expands(resultExpandable)

	return err
}

type GetTransactionResult struct {
	GetByIDRequest
}
