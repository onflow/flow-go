package models

import "github.com/onflow/flow-go/model/flow"

// LinkGenerator generates the expandable value for the known endpoints
// e.g. "/v1/blocks/c5e935bc75163db82e4a6cf9dc3b54656709d3e21c87385138300abd479c33b7"
type LinkGenerator interface {
	BlockLink(id flow.Identifier) (string, error)
	TransactionLink(id flow.Identifier) (string, error)
	TransactionResultLink(id flow.Identifier) (string, error)
	PayloadLink(id flow.Identifier) (string, error)
	ExecutionResultLink(id flow.Identifier) (string, error)
	AccountLink(address string) (string, error)
	CollectionLink(id flow.Identifier) (string, error)
}

func (l *Links) Build(link string, err error) error {
	if err != nil {
		return err
	}

	l.Self = link
	return nil
}
