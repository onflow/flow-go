package p2p

import (
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
)

// PeerstoreIdentifierProvider implements an IdentifierProvider which provides the identifiers
// of the peers present in the given LibP2P host's peerstore.
type PeerstoreIdentifierProvider struct {
	host         host.Host
	idTranslator IDTranslator
	logger       zerolog.Logger
}

func NewPeerstoreIdentifierProvider(logger zerolog.Logger, host host.Host, idTranslator IDTranslator) *PeerstoreIdentifierProvider {
	return &PeerstoreIdentifierProvider{
		logger:       logger.With().Str("component", "peerstore-id-provider").Logger(),
		host:         host,
		idTranslator: idTranslator,
	}
}

func (p *PeerstoreIdentifierProvider) Identifiers() flow.IdentifierList {
	var result flow.IdentifierList

	// get all peers with addresses from the peerstore
	pids := p.host.Peerstore().PeersWithAddrs()

	for _, pid := range pids {
		flowID, err := p.idTranslator.GetFlowID(pid)
		if err != nil {
			p.logger.Err(err).Str("peerID", pid.Pretty()).Msg("failed to translate to Flow ID")
			continue
		}

		result = append(result, flowID)
	}

	return result
}
