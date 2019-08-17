package intervalst

type node struct {
	interval    Interval
	value       interface{}
	left, right *node
	// size of subtree rooted at this node
	n int
	// max endpoint in subtree rooted at this node
	max Position
}

func newNode(interval Interval, value interface{}) *node {
	return &node{
		interval: interval,
		value:    value,
		n:        1,
		max:      interval.Max,
	}
}

func (n *node) size() int {
	if n == nil {
		return 0
	}
	return n.n
}

type MaxPosition struct{}

func (MaxPosition) CompareTo(other Position) int {
	_, ok := other.(MaxPosition)
	if ok {
		return 0
	}
	return 1
}

var maxPosition = MaxPosition{}

func (n *node) Max() Position {
	if n == nil {
		return maxPosition
	}

	return n.max
}

func (n *node) fix() {
	if n == nil {
		return
	}

	n.n = 1 + n.left.size() + n.right.size()
	n.max = max3(n.interval.Max, n.left.Max(), n.right.Max())
}

func max3(a, b, c Position) Position {
	if c.CompareTo(a) > 0 && c.CompareTo(b) > 0 {
		return c
	}
	if b.CompareTo(a) > 0 && b.CompareTo(c) > 0 {
		return b
	}
	return a
}

func (n *node) rotR() *node {
	x := n.left
	n.left = x.right
	x.right = n
	n.fix()
	x.fix()
	return x
}

func (n *node) rotL() *node {
	x := n.right
	n.right = x.left
	x.left = n
	n.fix()
	x.fix()
	return x
}

func (n *node) Values() []interface{} {
	if n == nil {
		return nil
	}

	return append(
		append(n.left.Values(), n.right.Values()...),
		n.value,
	)
}

func (n *node) checkCount() bool {
	return n == nil ||
		(n.left.checkCount() && n.right.checkCount() &&
			(n.n == 1+n.left.size()+n.right.size()))
}

func (n *node) checkMax() bool {
	if n == nil {
		return true
	}
	actual := max3(n.interval.Max, n.left.Max(), n.right.Max())
	return n.max.CompareTo(actual) == 0
}
