package p2p

import (
	"github.com/libp2p/go-libp2p-core/host"

	"github.com/onflow/flow-go/model/flow"
)

type PeerstoreIdentifierProvider struct {
	host         host.Host
	idTranslator IDTranslator
}

func NewPeerstoreIdentifierProvider(host host.Host, idTranslator IDTranslator) *PeerstoreIdentifierProvider {
	return &PeerstoreIdentifierProvider{host: host, idTranslator: idTranslator}
}

func (p *PeerstoreIdentifierProvider) Identifiers() flow.IdentifierList {
	var result flow.IdentifierList

	pids := p.host.Peerstore().PeersWithAddrs() // TODO: should we just call Peers here?
	for _, pid := range pids {
		flowID, err := p.idTranslator.GetFlowID(pid)
		if err != nil {
			// TODO: log error
		} else {
			result = append(result, flowID)
		}
	}

	return result
}
