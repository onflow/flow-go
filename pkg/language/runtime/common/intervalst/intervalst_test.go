package intervalst

import (
	"math/rand"
	"testing"

	. "github.com/onsi/gomega"
)

func TestIntervalSTRandom(t *testing.T) {

	N := 10000

	// generate N random intervals
	st := &IntervalST{}
	var interval Interval
	for i := 0; i < N; i++ {
		min := interval.Max + rand.Intn(100)
		max := min + rand.Intn(50)
		interval = NewInterval(min, max)

		st.Put(interval, i)
	}

	if !st.check() {
		t.Fail()
	}
}

func TestIntervalST(t *testing.T) {
	RegisterTestingT(t)

	st := &IntervalST{}

	st.Put(NewInterval(2, 4), 100)

	interval, value := st.Search(1)
	Expect(interval).To(BeNil())
	Expect(value).To(BeNil())

	interval, value = st.Search(2)
	Expect(interval).To(Equal(&Interval{2, 4}))
	Expect(value).To(Equal(100))

	interval, value = st.Search(3)
	Expect(interval).To(Equal(&Interval{2, 4}))
	Expect(value).To(Equal(100))

	interval, value = st.Search(4)
	Expect(interval).To(Equal(&Interval{2, 4}))
	Expect(value).To(Equal(100))

	interval, value = st.Search(5)
	Expect(interval).To(BeNil())
	Expect(value).To(BeNil())

	st.Put(NewInterval(8, 8), 200)

	interval, value = st.Search(7)
	Expect(interval).To(BeNil())
	Expect(value).To(BeNil())

	interval, value = st.Search(8)
	Expect(interval).To(Equal(&Interval{8, 8}))
	Expect(value).To(Equal(200))

	interval, value = st.Search(9)
	Expect(interval).To(BeNil())
	Expect(value).To(BeNil())

	if !st.check() {
		t.Fail()
	}
}
