package inmem

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/invalid"
	"github.com/onflow/flow-go/state/protocol/seed"
)

type Epoch struct {
	enc EncodableEpoch
}

func (e Epoch) Counter() (uint64, error)   { return e.enc.Counter, nil }
func (e Epoch) FirstView() (uint64, error) { return e.enc.FirstView, nil }
func (e Epoch) FinalView() (uint64, error) { return e.enc.FinalView, nil }
func (e Epoch) InitialIdentities() (flow.IdentityList, error) {
	return e.enc.InitialIdentities, nil
}
func (e Epoch) RandomSource() ([]byte, error) { return e.enc.RandomSource, nil }

func (e Epoch) Seed(indices ...uint32) ([]byte, error) {
	return seed.FromRandomSource(indices, e.enc.RandomSource)
}

func (e Epoch) Clustering() (flow.ClusterList, error) {
	var clusters flow.ClusterList
	for _, cluster := range e.enc.Clusters {
		clusters = append(clusters, cluster.Members)
	}
	return clusters, nil
}

func (e Epoch) DKG() (protocol.DKG, error) {
	if e.enc.DKG != nil {
		return DKG{*e.enc.DKG}, nil
	}
	return nil, protocol.ErrEpochNotCommitted
}

func (e Epoch) Cluster(i uint) (protocol.Cluster, error) {
	if e.enc.Clusters != nil {
		if i >= uint(len(e.enc.Clusters)) {
			return nil, fmt.Errorf("no cluster with index %d", i)
		}
		return Cluster{e.enc.Clusters[i]}, nil
	}
	return nil, protocol.ErrEpochNotCommitted
}

type Epochs struct {
	enc EncodableEpochs
}

func (eq Epochs) Previous() protocol.Epoch {
	if eq.enc.Previous != nil {
		return Epoch{*eq.enc.Previous}
	}
	return invalid.NewEpoch(protocol.ErrNoPreviousEpoch)
}
func (eq Epochs) Current() protocol.Epoch {
	return Epoch{*eq.enc.Current}
}
func (eq Epochs) Next() protocol.Epoch {
	if eq.enc.Next != nil {
		return Epoch{*eq.enc.Next}
	}
	return invalid.NewEpoch(protocol.ErrNextEpochNotSetup)
}
