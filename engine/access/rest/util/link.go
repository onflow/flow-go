package util

type Link struct {
	Self string `json:"_self,omitempty"`
}

func (l *Link) Build(link string) {
	l.Self = link
}
