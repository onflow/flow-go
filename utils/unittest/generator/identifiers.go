package generator

import "github.com/dapperlabs/flow-go/model/flow"

type Identifiers struct {
	count int
}

func IdentifierGenerator() *Identifiers {
	return &Identifiers{1}
}

func newIdentifier(count int) flow.Identifier {
	var id flow.Identifier
	for i := range id {
		id[i] = uint8(count)
	}

	return id
}

func (g *Identifiers) New() flow.Identifier {
	id := newIdentifier(g.count + 1)
	g.count++
	return id
}
