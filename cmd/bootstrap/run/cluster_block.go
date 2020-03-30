package run

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
)

func GenerateGenesisClusterBlocks(n int) []cluster.Block {
	clusterBlocks := make([]cluster.Block, n)
	for i := range clusterBlocks {
		clusterBlocks[i] = GenerateGenesisClusterBlock(i)
	}
	return clusterBlocks
}

func GenerateGenesisClusterBlock(index int) cluster.Block {
	payload := cluster.Payload{
		Collection: flow.LightCollection{Transactions: nil},
	}
	header := flow.Header{
		ChainID:                 fmt.Sprintf("collector cluster %v", index),
		ParentID:                flow.ZeroID,
		Height:                  0,
		PayloadHash:             payload.Hash(),
		Timestamp:               flow.GenesisTime(),
		View:                    0,
		ParentSigners:           nil,
		ParentStakingSigs:       nil,
		ParentRandomBeaconSig:   nil,
		ProposerID:              flow.ZeroID,
		ProposerStakingSig:      nil,
		ProposerRandomBeaconSig: nil,
	}

	return cluster.Block{
		Header:  header,
		Payload: payload,
	}
}
