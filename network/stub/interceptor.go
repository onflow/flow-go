package stub

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type Matcher func(*PendingMessage) bool

func ChannelID(channelID uint8) Matcher {
	return func(m *PendingMessage) bool {
		return m.ChannelID == channelID
	}
}

func FromID(id flow.Identifier) Matcher {
	return func(m *PendingMessage) bool {
		return m.From == id
	}
}

type Interceptor struct {
	net      *Network
	matchers []Matcher
	handler  func(*PendingMessage)
}

func (i *Interceptor) Do(f func(*PendingMessage)) {
	i.handler = f
	i.net.addInterceptor(i.onMessage)
}

func (i *Interceptor) onMessage(m *PendingMessage) {
	for _, matcher := range i.matchers {
		if matcher(m) {
			i.handler(m)
			return
		}
	}
}
