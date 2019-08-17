package intervalst

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
)

type lineAndColumn struct {
	line   int
	column int
}

func (l lineAndColumn) CompareTo(other Position) int {
	if _, ok := other.(MaxPosition); ok {
		return -1
	}

	otherL, ok := other.(lineAndColumn)
	if !ok {
		panic(fmt.Sprintf("not a lineAndColumn: %#+v", other))
	}
	if l.line < otherL.line {
		return -1
	}
	if l.line > otherL.line {
		return 1
	}
	if l.column < otherL.column {
		return -1
	}
	if l.column > otherL.column {
		return 1
	}
	return 0
}

func TestIntervalST(t *testing.T) {
	RegisterTestingT(t)

	st := &IntervalST{}

	st.Put(
		NewInterval(
			lineAndColumn{2, 2},
			lineAndColumn{2, 4},
		),
		100,
	)

	interval, value := st.Search(lineAndColumn{1, 3})
	Expect(interval).To(BeNil())
	Expect(value).To(BeNil())

	interval, value = st.Search(lineAndColumn{2, 1})
	Expect(interval).To(BeNil())
	Expect(value).To(BeNil())

	interval, value = st.Search(lineAndColumn{2, 2})
	//Expect(interval).To(Equal(&Interval{2, 4}))
	Expect(value).To(Equal(100))

	interval, value = st.Search(lineAndColumn{2, 3})
	//Expect(interval).To(Equal(&Interval{2, 4}))
	Expect(value).To(Equal(100))

	interval, value = st.Search(lineAndColumn{2, 4})
	//Expect(interval).To(Equal(&Interval{2, 4}))
	Expect(value).To(Equal(100))

	interval, value = st.Search(lineAndColumn{2, 5})
	Expect(interval).To(BeNil())
	Expect(value).To(BeNil())

	st.Put(
		NewInterval(
			lineAndColumn{3, 8},
			lineAndColumn{3, 8},
		),
		200,
	)

	interval, value = st.Search(lineAndColumn{2, 8})
	Expect(interval).To(BeNil())
	Expect(value).To(BeNil())

	interval, value = st.Search(lineAndColumn{4, 8})
	Expect(interval).To(BeNil())
	Expect(value).To(BeNil())

	interval, value = st.Search(lineAndColumn{3, 7})
	Expect(interval).To(BeNil())
	Expect(value).To(BeNil())

	interval, value = st.Search(lineAndColumn{3, 8})
	Expect(interval).To(Equal(&Interval{
		lineAndColumn{3, 8},
		lineAndColumn{3, 8},
	}))
	Expect(value).To(Equal(200))

	interval, value = st.Search(lineAndColumn{3, 9})
	Expect(interval).To(BeNil())
	Expect(value).To(BeNil())

	if !st.check() {
		t.Fail()
	}
}
