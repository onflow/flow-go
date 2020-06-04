package node

type StorableNode struct {
	LIndex    uint64
	RIndex    uint64
	Height    uint16 // Height where the node is at
	Key       []byte
	Value     []byte
	HashValue []byte
}

func RebuildFromStorables(storableNodes []*StorableNode) []*Node {

	nodes := make([]*Node, len(storableNodes))

	for i := 1; i < len(storableNodes); i++ {
		storableNode := storableNodes[i]

		nodes[i] = &Node{
			lChild:    nil,
			rChild:    nil,
			height:    int(storableNode.Height),
			key:       storableNode.Key,
			value:     storableNode.Value,
			hashValue: storableNode.HashValue,
		}
	}

	//fix references now
	for i := 1; i < len(storableNodes); i++ {
		storableNode := storableNodes[i]

		if storableNode.LIndex > 0 {
			nodes[i].lChild = nodes[storableNode.LIndex]
		}

		if storableNode.RIndex > 0 {
			nodes[i].rChild = nodes[storableNode.RIndex]
		}
	}

	return nodes
}
