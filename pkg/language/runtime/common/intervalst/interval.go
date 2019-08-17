package intervalst

import "fmt"

type Interval struct {
	Min, Max int
}

func NewInterval(min, max int) Interval {
	if min > max {
		panic("illegal interval")
	}
	return Interval{min, max}
}

func (i Interval) Intersects(other Interval) bool {
	return !(other.Max < i.Min || i.Max < other.Min)
}

func (i Interval) Contains(x int) bool {
	return i.Min <= x && x <= i.Max
}

func (i Interval) CompareTo(other Interval) int {
	if i.Min < other.Min {
		return -1
	} else if i.Min > other.Min {
		return +1
	} else if i.Max < other.Max {
		return -1
	} else if i.Max > other.Max {
		return +1
	} else {
		return 0
	}
}

func (i Interval) String() string {
	return fmt.Sprintf("[%d, %d]", i.Min, i.Max)
}
