package serializable

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/seed"
)

type epoch struct {
	counter           uint64
	firstView         uint64
	finalView         uint64
	seed              []byte
	initialIdentities flow.IdentityList
	clustering        flow.ClusterList
	clusters          []cluster
	dkg               dkg
}

type epochQuery struct {
	prev *epoch
	curr *epoch
	next *epoch
}

func (e *epoch) Counter() (uint64, error) {
	return e.counter, nil
}

func (e *epoch) FinalView() (uint64, error) {
	return e.finalView, nil
}

func (e *epoch) Seed(indices ...uint32) ([]byte, error) {
	return seed.FromRandomSource(indices, e.seed)
}

func (e *epoch) InitialIdentities() (flow.IdentityList, error) {
	return e.initialIdentities, nil
}

func (e *epoch) Clustering() (flow.ClusterList, error) {
	return e.clustering, nil
}

func (e *epoch) Cluster(i uint) (protocol.Cluster, error) {
	return e.clusters[i], nil
}

func (e *epoch) DKG() (protocol.DKG, error) {
	return e.dkg
}

func (eq *epochQuery) Previous() protocol.Epoch {
	return eq.prev
}

func (eq *epochQuery) Current() protocol.Epoch {
	return eq.curr
}

func (eq *epochQuery) Next() protocol.Epoch {
	return eq.next
}
