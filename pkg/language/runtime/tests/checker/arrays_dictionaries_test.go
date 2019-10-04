package checker

import (
	"fmt"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCheckDictionary(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let z = {"a": 1, "b": 2}
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckDictionaryType(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let z: {String: Int} = {"a": 1, "b": 2}
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidDictionaryTypeKey(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let z: {Int: Int} = {"a": 1, "b": 2}
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidDictionaryTypeValue(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let z: {String: String} = {"a": 1, "b": 2}
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidDictionaryTypeSwapped(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let z: {Int: String} = {"a": 1, "b": 2}
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidDictionaryKeys(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let z = {"a": 1, true: 2}
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidDictionaryValues(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let z = {"a": 1, "b": true}
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckDictionaryIndexingString(t *testing.T) {
	RegisterTestingT(t)

	checker, err := ParseAndCheck(`
      let x = {"abc": 1, "def": 2}
      let y = x["abc"]
    `)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.GlobalValues["y"].Type).
		To(Equal(&sema.OptionalType{Type: &sema.IntType{}}))
}

func TestCheckDictionaryIndexingBool(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let x = {true: 1, false: 2}
      let y = x[true]
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidDictionaryIndexing(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let x = {"abc": 1, "def": 2}
      let y = x[true]
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotIndexingTypeError{}))
}

func TestCheckDictionaryIndexingAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          let x = {"abc": 1, "def": 2}
          x["abc"] = 3
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidDictionaryIndexingAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          let x = {"abc": 1, "def": 2}
          x["abc"] = true
      }
    `)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckDictionaryRemove(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          let x = {"abc": 1, "def": 2}
          x.remove(key: "abc")
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidDictionaryRemove(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          let x = {"abc": 1, "def": 2}
          x.remove(key: true)
      }
    `)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckLength(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let x = "cafe\u{301}".length
      let y = [1, 2, 3].length
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckArrayAppend(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.append(4)
          return x
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidArrayAppend(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.append("4")
          return x
      }
    `)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckArrayAppendBound(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): [Int] {
          let x = [1, 2, 3]
          let y = x.append
          y(4)
          return x
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckArrayConcat(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
	  fun test(): [Int] {
	 	  let a = [1, 2]
		  let b = [3, 4]
          let c = a.concat(b)
          return c
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidArrayConcat(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): [Int] {
		  let a = [1, 2]
		  let b = ["a", "b"]
          let c = a.concat(b)
          return c
      }
    `)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckArrayConcatBound(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): [Int] {
		  let a = [1, 2]
		  let b = [3, 4]
		  let c = a.concat
		  return c(b)
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckArrayInsert(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.insert(at: 1, 4)
          return x
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidArrayInsert(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.insert(at: 1, "4")
          return x
      }
    `)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckArrayRemove(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.remove(at: 1)
          return x
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidArrayRemove(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.remove(at: "1")
          return x
      }
    `)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckArrayRemoveFirst(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.removeFirst()
          return x
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidArrayRemoveFirst(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.removeFirst(1)
          return x
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.ArgumentCountError{}))
}

func TestCheckArrayRemoveLast(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.removeLast()
          return x
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckArrayContains(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): Bool {
          let x = [1, 2, 3]
          return x.contains(2)
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidArrayContains(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): Bool {
          let x = [1, 2, 3]
          return x.contains("abc")
      }
    `)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidArrayContainsNotEquatable(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): Bool {
          let z = [[1], [2], [3]]
          return z.contains([1, 2])
      }
    `)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotEquatableTypeError{}))
}

func TestCheckEmptyArray(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let xs: [Int] = []
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckEmptyArrayCall(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun foo(xs: [Int]) {
          foo(xs: [])
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckEmptyDictionary(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let xs: {String: Int} = {}
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckEmptyDictionaryCall(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun foo(xs: {String: Int}) {
          foo(xs: {})
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckArraySubtyping(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
              %[1]s interface I {}
              %[1]s S: I {}

              let xs: %[2]s[S] %[3]s []
              let ys: %[2]s[I] %[3]s xs
	        `,
				kind.Keyword(),
				kind.Annotation(),
				kind.TransferOperator(),
			))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				Expect(err).
					To(Not(HaveOccurred()))
			} else {
				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInvalidArraySubtyping(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let xs: [Bool] = []
      let ys: [Int] = xs
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckDictionarySubtyping(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
              %[1]s interface I {}
              %[1]s S: I {}

              let xs: %[2]s{String: S} %[3]s {}
              let ys: %[2]s{String: I} %[3]s xs
	        `,
				kind.Keyword(),
				kind.Annotation(),
				kind.TransferOperator(),
			))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				Expect(err).
					To(Not(HaveOccurred()))
			} else {
				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInvalidDictionarySubtyping(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let xs: {String: Bool} = {}
      let ys: {String: Int} = xs
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidArrayElements(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let z = [0, true]
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}
