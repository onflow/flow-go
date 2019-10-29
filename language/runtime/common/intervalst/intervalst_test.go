package intervalst

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

type lineAndColumn struct {
	Line   int
	Column int
}

func (l lineAndColumn) Compare(other Position) int {
	if _, ok := other.(MinPosition); ok {
		return 1
	}

	otherL, ok := other.(lineAndColumn)
	if !ok {
		panic(fmt.Sprintf("not a lineAndColumn: %#+v", other))
	}
	if l.Line < otherL.Line {
		return -1
	}
	if l.Line > otherL.Line {
		return 1
	}
	if l.Column < otherL.Column {
		return -1
	}
	if l.Column > otherL.Column {
		return 1
	}
	return 0
}

func TestIntervalST(t *testing.T) {

	st := &IntervalST{}

	st.Put(
		NewInterval(
			lineAndColumn{2, 2},
			lineAndColumn{2, 4},
		),
		100,
	)

	interval, value := st.Search(lineAndColumn{1, 3})
	assert.Nil(t, interval)
	assert.Nil(t, value)

	interval, value = st.Search(lineAndColumn{2, 1})
	assert.Nil(t, interval)
	assert.Nil(t, value)

	interval, value = st.Search(lineAndColumn{2, 2})
	assert.Equal(t, interval, &Interval{
		lineAndColumn{2, 2},
		lineAndColumn{2, 4},
	})
	assert.Equal(t, value, 100)

	interval, value = st.Search(lineAndColumn{2, 3})
	assert.Equal(t, interval, &Interval{
		lineAndColumn{2, 2},
		lineAndColumn{2, 4},
	})
	assert.Equal(t, value, 100)

	interval, value = st.Search(lineAndColumn{2, 4})
	assert.Equal(t, interval, &Interval{
		lineAndColumn{2, 2},
		lineAndColumn{2, 4},
	})
	assert.Equal(t, value, 100)

	interval, value = st.Search(lineAndColumn{2, 5})
	assert.Nil(t, interval)
	assert.Nil(t, value)

	st.Put(
		NewInterval(
			lineAndColumn{3, 8},
			lineAndColumn{3, 8},
		),
		200,
	)

	interval, value = st.Search(lineAndColumn{2, 8})
	assert.Nil(t, interval)
	assert.Nil(t, value)

	interval, value = st.Search(lineAndColumn{4, 8})
	assert.Nil(t, interval)
	assert.Nil(t, value)

	interval, value = st.Search(lineAndColumn{3, 7})
	assert.Nil(t, interval)
	assert.Nil(t, value)

	interval, value = st.Search(lineAndColumn{3, 8})
	assert.Equal(t, interval, &Interval{
		lineAndColumn{3, 8},
		lineAndColumn{3, 8},
	})
	assert.Equal(t, value, 200)

	interval, value = st.Search(lineAndColumn{3, 9})
	assert.Nil(t, interval)
	assert.Nil(t, value)

	if !st.check() {
		t.Fail()
	}
}

func TestIntervalST2(t *testing.T) {

	intervals := []Interval{
		{
			lineAndColumn{Line: 2, Column: 12},
			lineAndColumn{Line: 2, Column: 12},
		},
		{
			lineAndColumn{Line: 3, Column: 12},
			lineAndColumn{Line: 3, Column: 12},
		},
		{
			lineAndColumn{Line: 5, Column: 12},
			lineAndColumn{Line: 5, Column: 13},
		},
		{
			lineAndColumn{Line: 5, Column: 15},
			lineAndColumn{Line: 5, Column: 20},
		},
		{
			lineAndColumn{Line: 5, Column: 28},
			lineAndColumn{Line: 5, Column: 33},
		},
		{
			lineAndColumn{Line: 6, Column: 15},
			lineAndColumn{Line: 6, Column: 15},
		},
		{
			lineAndColumn{Line: 7, Column: 15},
			lineAndColumn{Line: 7, Column: 15},
		},
		{
			lineAndColumn{Line: 7, Column: 25},
			lineAndColumn{Line: 7, Column: 25},
		},
		{
			lineAndColumn{Line: 8, Column: 15},
			lineAndColumn{Line: 8, Column: 16},
		},
		{
			lineAndColumn{Line: 9, Column: 21},
			lineAndColumn{Line: 9, Column: 21},
		},
		{
			lineAndColumn{Line: 9, Column: 25},
			lineAndColumn{Line: 9, Column: 25},
		},
		{
			lineAndColumn{Line: 14, Column: 15},
			lineAndColumn{Line: 14, Column: 16},
		},
		{
			lineAndColumn{Line: 15, Column: 16},
			lineAndColumn{Line: 15, Column: 19},
		},
		{
			lineAndColumn{Line: 18, Column: 18},
			lineAndColumn{Line: 18, Column: 19},
		},
		{
			lineAndColumn{Line: 20, Column: 12},
			lineAndColumn{Line: 20, Column: 13},
		},
		{
			lineAndColumn{Line: 21, Column: 11},
			lineAndColumn{Line: 21, Column: 12},
		},
		{
			lineAndColumn{Line: 22, Column: 18},
			lineAndColumn{Line: 22, Column: 19},
		},
	}

	st := &IntervalST{}

	rand.Shuffle(len(intervals), func(i, j int) {
		intervals[i], intervals[j] = intervals[j], intervals[i]
	})

	for _, interval := range intervals {
		st.Put(interval, interval)
	}

	if !st.check() {
		t.Fail()
	}

	for _, interval := range intervals {
		res, _ := st.Search(interval.Min)
		assert.NotNil(t, res)
		res, _ = st.Search(interval.Max)
		assert.NotNil(t, res)
	}

	for _, value := range st.Values() {
		interval := value.(Interval)
		res, _ := st.Search(interval.Min)
		assert.NotNil(t, res)
		res, _ = st.Search(interval.Max)
		assert.NotNil(t, res)
	}
}
