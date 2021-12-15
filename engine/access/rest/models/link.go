package models

type LinkResponse struct {
	Self string `json:"_self,omitempty"`
}

func (l *LinkResponse) Build(link string, err error) error {
	if err != nil {
		return err
	}

	l.Self = link
	return nil
}
