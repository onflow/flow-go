package response

import "github.com/onflow/flow-go/engine/access/rest/models"

type Links models.Links

func (l *Links) Build(link string, err error) error {
	if err != nil {
		return err
	}

	l.Self = link
	return nil
}
