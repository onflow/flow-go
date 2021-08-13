package p2p

import (
	peerstore "github.com/libp2p/go-libp2p-peerstore"

	"github.com/onflow/flow-go/model/flow"
)

type PeerstoreIdentifierProvider struct {
	store        peerstore.Peerstore
	idTranslator IDTranslator
}

func NewPeerstoreIdentifierProvider(store peerstore.Peerstore, idTranslator IDTranslator) *PeerstoreIdentifierProvider {
	return &PeerstoreIdentifierProvider{store: store, idTranslator: idTranslator}
}

func (p *PeerstoreIdentifierProvider) Identifiers() flow.IdentifierList {
	var result flow.IdentifierList

	pids := p.store.PeersWithAddrs() // TODO: should we just call Peers here?
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
