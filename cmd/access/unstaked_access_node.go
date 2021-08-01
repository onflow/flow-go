package main

import "github.com/onflow/flow-go/module"

type ConsensusFollower interface {
	module.ReadyDoneAware
}
type UnstakedAccessNode struct {

}

func (unstakedAN *UnstakedAccessNode) Ready() <-chan struct{} {

}

func (unstakedAN *UnstakedAccessNode) Done() <-chan struct{} {

}
