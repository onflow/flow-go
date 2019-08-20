package intervalst

import "math/rand"

// IntervalST

type IntervalST struct {
	root *node
}

func (t *IntervalST) Get(interval Interval) interface{} {
	return t.get(t.root, interval)
}

func (t *IntervalST) get(x *node, interval Interval) interface{} {
	if x == nil {
		return nil
	}
	switch cmp := interval.CompareTo(x.interval); {
	case cmp < 0:
		return t.get(x.left, interval)
	case cmp > 0:
		return t.get(x.right, interval)
	default:
		return x.value
	}
}

func (t *IntervalST) Contains(interval Interval) bool {
	return t.Get(interval) != nil
}

// Put associates an interval with a value.
//
// NOTE: does *not* check if the interval already exists
//
func (t *IntervalST) Put(interval Interval, value interface{}) {
	t.root = t.randomizedInsert(t.root, interval, value)
}

func (t *IntervalST) randomizedInsert(x *node, interval Interval, value interface{}) *node {
	if x == nil {
		return newNode(interval, value)
	}

	if rand.Float32()*float32(x.size()) < 1.0 {
		return t.rootInsert(x, interval, value)
	}

	cmp := interval.CompareTo(x.interval)
	if cmp < 0 {
		x.left = t.randomizedInsert(x.left, interval, value)
	} else {
		x.right = t.randomizedInsert(x.right, interval, value)
	}

	x.fix()

	return x
}

func (t *IntervalST) rootInsert(x *node, interval Interval, value interface{}) *node {
	if x == nil {
		return newNode(interval, value)
	}

	cmp := interval.CompareTo(x.interval)
	if cmp < 0 {
		x.left = t.rootInsert(x.left, interval, value)
		x = x.rotR()
	} else {
		x.right = t.rootInsert(x.right, interval, value)
		x = x.rotL()
	}

	return x
}

func (t *IntervalST) SearchInterval(interval Interval) (*Interval, interface{}) {
	return t.searchInterval(t.root, interval)
}

func (t *IntervalST) searchInterval(x *node, interval Interval) (*Interval, interface{}) {
	for x != nil {
		if x.interval.Intersects(interval) {
			return &x.interval, x.value
		} else if x.left == nil || x.left.max.CompareTo(interval.Min) < 0 {
			x = x.right
		} else {
			x = x.left
		}
	}
	return nil, nil
}

func (t *IntervalST) Search(p Position) (*Interval, interface{}) {
	return t.search(t.root, p)
}

func (t *IntervalST) search(x *node, p Position) (*Interval, interface{}) {
	for x != nil {
		if x.interval.Contains(p) {
			return &x.interval, x.value
		} else if x.left == nil || x.left.max.CompareTo(p) < 0 {
			x = x.right
		} else {
			x = x.left
		}
	}
	return nil, nil
}

func (t *IntervalST) Values() []interface{} {
	return t.root.Values()
}

func (t *IntervalST) check() bool {
	return t.root.checkCount() && t.root.checkMax()
}
