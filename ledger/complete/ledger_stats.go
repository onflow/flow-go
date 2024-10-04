package complete

import (
	"github.com/schollz/progressbar/v3"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
)

type LedgerStats struct {
	TrieCount        uint64 `json:"tries_count"`
	NodeCount        uint64 `json:"node_count"`
	InterimNodeCount uint64 `json:"interim_node_count"`
	LeafNodeCount    uint64 `json:"leaf_node_count"`
}

func (l *Ledger) CollectStats(payloadCallBack func(payload *ledger.Payload)) (*LedgerStats, error) {
	tries, err := l.Tries()
	if err != nil {
		return nil, err
	}

	return CollectStats(tries, payloadCallBack)
}

func CollectStats(tries []*trie.MTrie, payloadCallBack func(payload *ledger.Payload)) (*LedgerStats, error) {
	visitedNodes := make(map[*node.Node]uint64)
	var interimNodeCounter, leafNodeCounter, totalNodeCounter uint64

	bar := progressbar.Default(int64(len(tries)), "collecting ledger stats")
	for _, trie := range tries {
		for itr := flattener.NewUniqueNodeIterator(trie.RootNode(), visitedNodes); itr.Next(); {
			n := itr.Value()
			if n.IsLeaf() {
				payload := n.Payload()
				leafNodeCounter++
				payloadCallBack(payload)
			} else {
				interimNodeCounter++
			}
			visitedNodes[n] = totalNodeCounter
			totalNodeCounter++
		}
		if err := bar.Add(1); err != nil {
			return nil, err
		}
	}

	return &LedgerStats{
		TrieCount:        uint64(len(tries)),
		NodeCount:        totalNodeCounter,
		InterimNodeCount: interimNodeCounter,
		LeafNodeCount:    leafNodeCounter,
	}, nil
}
