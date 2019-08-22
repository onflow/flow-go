package intervalst

import "fmt"

type Position interface {
	CompareTo(other Position) int
}

type Interval struct {
	Min, Max Position
}

func NewInterval(min, max Position) Interval {
	if min.CompareTo(max) > 0 {
		panic("illegal interval: min > max")
	}
	return Interval{min, max}
}

func (i Interval) Intersects(other Interval) bool {
	return !(other.Max.CompareTo(i.Min) == -1 ||
		i.Max.CompareTo(other.Min) == -1)
}

func (i Interval) Contains(x Position) bool {
	return i.Min.CompareTo(x) <= 0 &&
		x.CompareTo(i.Max) <= 0
}

func (i Interval) CompareTo(other Interval) int {
	mins := i.Min.CompareTo(other.Min)
	maxs := i.Max.CompareTo(other.Max)
	if mins < 0 {
		return -1
	} else if mins > 0 {
		return 1
	} else if maxs < 0 {
		return -1
	} else if maxs > 0 {
		return 1
	} else {
		return 0
	}
}

func (i Interval) String() string {
	return fmt.Sprintf("[%s, %s]", i.Min, i.Max)
}
