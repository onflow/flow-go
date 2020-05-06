package mtrie

type MTrie struct {
	root   *node
	parent *MTrie
	values map[string][]byte
}

func NewMTrie(root *node) *MTrie {
	return &MTrie{root: root, parent: nil, values: make(map[string][]byte)}
}
