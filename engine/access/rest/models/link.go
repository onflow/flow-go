package models

func (l *Links) Build(link string, err error) error {
	if err != nil {
		return err
	}

	l.Self = link
	return nil
}
