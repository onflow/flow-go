package p2p

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/onflow/flow-go/model/flow"
)

// HierarchicalIDTranslator implements an IDTranslator which combines the ID translation
// capabilities of multiple IDTranslators.
// When asked to translate an ID, it will iterate through all of the IDTranslators it was
// given and return the first successful translation.
type HierarchicalIDTranslator struct {
	translators []IDTranslator
}

func NewHierarchicalIDTranslator(translators ...IDTranslator) *HierarchicalIDTranslator {
	return &HierarchicalIDTranslator{translators}
}

func (t *HierarchicalIDTranslator) GetPeerID(flowID flow.Identifier) (peer.ID, error) {
	var errs *multierror.Error
	for _, translator := range t.translators {
		pid, err := translator.GetPeerID(flowID)
		if err == nil {
			return pid, nil
		}
		errs = multierror.Append(errs, err)
	}
	return "", fmt.Errorf("could not translate the given flow ID: %w", errs)
}

func (t *HierarchicalIDTranslator) GetFlowID(peerID peer.ID) (flow.Identifier, error) {
	var errs *multierror.Error
	for _, translator := range t.translators {
		fid, err := translator.GetFlowID(peerID)
		if err == nil {
			return fid, nil
		}
		errs = multierror.Append(errs, err)
	}
	return flow.ZeroID, fmt.Errorf("could not translate the given peer ID: %w", errs)
}
