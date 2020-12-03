package inmem

import (
	"fmt"

	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/seed"
)

type Epoch struct {
	encodable.Epoch
}

func (e Epoch) Counter() (uint64, error)                      { return e.Epoch.Counter, nil }
func (e Epoch) FirstView() (uint64, error)                    { return e.Epoch.FirstView, nil }
func (e Epoch) FinalView() (uint64, error)                    { return e.Epoch.FinalView, nil }
func (e Epoch) InitialIdentities() (flow.IdentityList, error) { return e.Epoch.InitialIdentities, nil }
func (e Epoch) DKG() (protocol.DKG, error)                    { return dkg{e.Epoch.DKG}, nil }

func (e Epoch) Seed(indices ...uint32) ([]byte, error) {
	return seed.FromRandomSource(indices, e.Epoch.Seed)
}

func (e Epoch) Clustering() (flow.ClusterList, error) {
	var clusters flow.ClusterList
	for _, cluster := range e.Epoch.Clusters {
		clusters = append(clusters, cluster.Members)
	}
	return clusters, nil
}

func (e Epoch) Cluster(i uint) (protocol.Cluster, error) {
	if i >= uint(len(e.Epoch.Clusters)) {
		return nil, fmt.Errorf("no cluster with index %d", i)
	}
	return Cluster{e.Epoch.Clusters[i]}, nil
}

type Epochs struct {
	encodable.Epochs
}

func (eq Epochs) Previous() protocol.Epoch { return Epoch{eq.Epochs.Previous} }
func (eq Epochs) Current() protocol.Epoch  { return Epoch{eq.Epochs.Current} }
func (eq Epochs) Next() protocol.Epoch     { return Epoch{eq.Epochs.Next} }
