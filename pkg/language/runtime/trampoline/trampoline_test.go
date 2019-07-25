package trampoline

import (
	. "github.com/onsi/gomega"
	"strconv"
	"testing"
)

func TestFlatMap(t *testing.T) {
	RegisterTestingT(t)

	trampoline :=
		More(func() Trampoline { return Done{23} }).
			FlatMap(func(value interface{}) Trampoline {
				number := value.(int)
				return Done{number * 42}
			})

	Expect(Run(trampoline)).To(Equal(23 * 42))
}

func TestFlatMap2(t *testing.T) {
	RegisterTestingT(t)

	trampoline :=
		More(func() Trampoline { return Done{23} }).
			FlatMap(func(value interface{}) Trampoline {
				n := value.(int)
				return More(func() Trampoline {
					return Done{strconv.Itoa(n)}
				})
			}).
			FlatMap(func(value interface{}) Trampoline {
				str := value.(string)
				return Done{str + "42"}
			})

	Expect(Run(trampoline)).To(Equal("2342"))
}

func TestFlatMap3(t *testing.T) {
	RegisterTestingT(t)

	trampoline :=
		More(func() Trampoline {
			return Done{23}.
				FlatMap(func(value interface{}) Trampoline {
					n := value.(int)
					return Done{n * 42}
				})
		}).
			FlatMap(func(value interface{}) Trampoline {
				n := value.(int)
				return Done{strconv.Itoa(n)}
			})

	Expect(Run(trampoline)).To(Equal(strconv.Itoa(23 * 42)))
}

func TestMap(t *testing.T) {
	RegisterTestingT(t)

	trampoline :=
		More(func() Trampoline { return Done{23} }).
			Map(func(value interface{}) interface{} {
				n := value.(int)
				return n * 42
			})

	Expect(Run(trampoline)).To(Equal(23 * 42))
}

func TestMap2(t *testing.T) {
	RegisterTestingT(t)

	trampoline :=
		Done{23}.
			Map(func(value interface{}) interface{} {
				n := value.(int)
				return n * 42
			})

	Expect(Run(trampoline)).To(Equal(23 * 42))
}

func TestEvenOdd(t *testing.T) {
	RegisterTestingT(t)

	var even, odd func(n interface{}) Trampoline

	even = func(value interface{}) Trampoline {
		n := value.(int)
		if n == 0 {
			return Done{true}
		}

		return More(func() Trampoline {
			return odd(n - 1)
		})
	}

	odd = func(value interface{}) Trampoline {
		n := value.(int)
		if n == 0 {
			return Done{false}
		}

		return More(func() Trampoline {
			return even(n - 1)
		})
	}

	Expect(Run(odd(99999))).To(BeTrue())
	Expect(Run(even(100000))).To(BeTrue())
	Expect(Run(odd(100000))).To(BeFalse())
	Expect(Run(even(99999))).To(BeFalse())
}

func TestAckermann(t *testing.T) {
	RegisterTestingT(t)

	// The recursive implementation of the Ackermann function
	// results in a stack overflow even for small inputs:
	//
	//  func ackermann(m, n int) int {
	//  	if m <= 0 {
	//  		return n + 1
	//  	}
	//
	//  	if n <= 0 {
	//  		return ackermann(m-1, 1)
	//  	}
	//
	//  	x := ackermann(m, n-1)
	//  	return ackermann(m-1, x)
	//  }
	//
	// The following version uses trampolines to avoid
	// the overflow:

	var ackermann func(m, n int) Trampoline

	ackermann = func(m, n int) Trampoline {
		if m <= 0 {
			return Done{n + 1}
		}
		if n <= 0 {
			return More(func() Trampoline {
				return ackermann(m-1, 1)
			})
		}
		first := More(func() Trampoline {
			return ackermann(m, n-1)
		})
		second := func(value interface{}) Trampoline {
			x := value.(int)
			return More(func() Trampoline {
				return ackermann(m-1, x)
			})
		}
		return first.FlatMap(second)
	}

	Expect(Run(ackermann(1, 2))).To(Equal(4))
	Expect(Run(ackermann(3, 2))).To(Equal(29))
	Expect(Run(ackermann(3, 4))).To(Equal(125))
	Expect(Run(ackermann(3, 7))).To(Equal(1021))
}
