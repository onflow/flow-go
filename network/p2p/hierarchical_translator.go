package p2p

import (
	"fmt"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/onflow/flow-go/model/flow"
)

type HierarchicalIDTranslator struct {
	translators []IDTranslator
}

func NewHierarchicalIDTranslator(translators ...IDTranslator) *HierarchicalIDTranslator {
	return &HierarchicalIDTranslator{translators}
}

func (t *HierarchicalIDTranslator) GetPeerID(flowID flow.Identifier) (peer.ID, error) {
	for _, translator := range t.translators {
		pid, err := translator.GetPeerID(flowID)
		if err == nil {
			return pid, nil
		}
	}
	return "", fmt.Errorf("could not find corresponding peer ID for flow ID %v", flowID)
}

func (t *HierarchicalIDTranslator) GetFlowID(peerID peer.ID) (flow.Identifier, error) {
	for _, translator := range t.translators {
		fid, err := translator.GetFlowID(peerID)
		if err == nil {
			return fid, nil
		}
	}
	return flow.ZeroID, fmt.Errorf("could not find corresponding flow ID for peer ID %v", peerID)
}
