package checker

import (
	"fmt"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckDictionary(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let z = {"a": 1, "b": 2}
	`)

	assert.Nil(t, err)
}

func TestCheckDictionaryType(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let z: {String: Int} = {"a": 1, "b": 2}
	`)

	assert.Nil(t, err)
}

func TestCheckInvalidDictionaryTypeKey(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let z: {Int: Int} = {"a": 1, "b": 2}
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidDictionaryTypeValue(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let z: {String: String} = {"a": 1, "b": 2}
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidDictionaryTypeSwapped(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let z: {Int: String} = {"a": 1, "b": 2}
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidDictionaryKeys(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let z = {"a": 1, true: 2}
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidDictionaryValues(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let z = {"a": 1, "b": true}
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckDictionaryIndexingString(t *testing.T) {

	checker, err := ParseAndCheck(t, `
      let x = {"abc": 1, "def": 2}
      let y = x["abc"]
    `)

	assert.Nil(t, err)

	assert.Equal(t,
		&sema.OptionalType{Type: &sema.IntType{}},
		checker.GlobalValues["y"].Type,
	)
}

func TestCheckDictionaryIndexingBool(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x = {true: 1, false: 2}
      let y = x[true]
	`)

	assert.Nil(t, err)
}

func TestCheckInvalidDictionaryIndexing(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x = {"abc": 1, "def": 2}
      let y = x[true]
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotIndexingTypeError{}, errs[0])
}

func TestCheckDictionaryIndexingAssignment(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          let x = {"abc": 1, "def": 2}
          x["abc"] = 3
      }
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidDictionaryIndexingAssignment(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          let x = {"abc": 1, "def": 2}
          x["abc"] = true
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckDictionaryRemove(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          let x = {"abc": 1, "def": 2}
          let old: Int? = x.remove(key: "abc")
      }
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidDictionaryRemove(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          let x = {"abc": 1, "def": 2}
          let old: Int? = x.remove(key: true)
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckDictionaryInsert(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          let x = {"abc": 1, "def": 2}
          let old: Int? = x.insert(key: "abc", 3)
      }
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidDictionaryInsert(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          let x = {"abc": 1, "def": 2}
          let old: Int? = x.insert(key: true, 3)
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckLength(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x = "cafe\u{301}".length
      let y = [1, 2, 3].length
    `)

	assert.Nil(t, err)
}

func TestCheckArrayAppend(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.append(4)
          return x
      }
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidArrayAppend(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.append("4")
          return x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckArrayAppendBound(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): [Int] {
          let x = [1, 2, 3]
          let y = x.append
          y(4)
          return x
      }
    `)

	assert.Nil(t, err)
}

func TestCheckArrayConcat(t *testing.T) {

	_, err := ParseAndCheck(t, `
	  fun test(): [Int] {
	 	  let a = [1, 2]
		  let b = [3, 4]
          let c = a.concat(b)
          return c
      }
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidArrayConcat(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): [Int] {
		  let a = [1, 2]
		  let b = ["a", "b"]
          let c = a.concat(b)
          return c
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckArrayConcatBound(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): [Int] {
		  let a = [1, 2]
		  let b = [3, 4]
		  let c = a.concat
		  return c(b)
      }
    `)

	assert.Nil(t, err)
}

func TestCheckArrayInsert(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.insert(at: 1, 4)
          return x
      }
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidArrayInsert(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): [Int] {
          let x = [1, 2, 3]
          x.insert(at: 1, "4")
          return x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckArrayRemove(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): [Int] {
          let x = [1, 2, 3]
          let old: Int? = x.remove(at: 1)
          return x
      }
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidArrayRemove(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): [Int] {
          let x = [1, 2, 3]
          let old: Int? = x.remove(at: "1")
          return x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckArrayRemoveFirst(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): [Int] {
          let x = [1, 2, 3]
          let old: Int? = x.removeFirst()
          return x
      }
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidArrayRemoveFirst(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): [Int] {
          let x = [1, 2, 3]
          let old: Int? = x.removeFirst(1)
          return x
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ArgumentCountError{}, errs[0])
}

func TestCheckArrayRemoveLast(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): [Int] {
          let x = [1, 2, 3]
          let old: Int? = x.removeLast()
          return x
      }
    `)

	assert.Nil(t, err)
}

func TestCheckArrayContains(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): Bool {
          let x = [1, 2, 3]
          return x.contains(2)
      }
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidArrayContains(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): Bool {
          let x = [1, 2, 3]
          return x.contains("abc")
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidArrayContainsNotEquatable(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): Bool {
          let z = [[1], [2], [3]]
          return z.contains([1, 2])
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotEquatableTypeError{}, errs[0])
}

func TestCheckEmptyArray(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let xs: [Int] = []
	`)

	assert.Nil(t, err)
}

func TestCheckEmptyArrayCall(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun foo(xs: [Int]) {
          foo(xs: [])
      }
	`)

	assert.Nil(t, err)
}

func TestCheckEmptyDictionary(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let xs: {String: Int} = {}
	`)

	assert.Nil(t, err)
}

func TestCheckEmptyDictionaryCall(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun foo(xs: {String: Int}) {
          foo(xs: {})
      }
	`)

	assert.Nil(t, err)
}

func TestCheckArraySubtyping(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s interface I {}
              %[1]s S: I {}

              let xs: %[2]s[S] %[3]s []
              let ys: %[2]s[I] %[3]s xs
	        `,
				kind.Keyword(),
				kind.Annotation(),
				kind.TransferOperator(),
			))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				assert.Nil(t, err)

			default:
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}

func TestCheckInvalidArraySubtyping(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let xs: [Bool] = []
      let ys: [Int] = xs
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckDictionarySubtyping(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s interface I {}
              %[1]s S: I {}

              let xs: %[2]s{String: S} %[3]s {}
              let ys: %[2]s{String: I} %[3]s xs
	        `,
				kind.Keyword(),
				kind.Annotation(),
				kind.TransferOperator(),
			))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				assert.Nil(t, err)

			default:
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}

func TestCheckInvalidDictionarySubtyping(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let xs: {String: Bool} = {}
      let ys: {String: Int} = xs
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidArrayElements(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let z = [0, true]
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidArrayIndexingWithType(t *testing.T) {

	_, err := ParseAndCheckWithExtra(t,
		`
          let x = ["xyz"][String?]
	    `,
		storageValueDeclaration,
		nil,
		nil,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidIndexingError{}, errs[0])
}

func TestCheckInvalidArrayIndexingAssignmentWithType(t *testing.T) {

	_, err := ParseAndCheckWithExtra(t,
		`
          fun test() {
              let stuff = ["abc"]
              stuff[String?] = "xyz"
          }
	    `,
		storageValueDeclaration,
		nil,
		nil,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidIndexingError{}, errs[0])
}

func TestCheckInvalidDictionaryIndexingWithType(t *testing.T) {

	_, err := ParseAndCheckWithExtra(t,
		`
          let x = {"a": 1}[String?]
	    `,
		storageValueDeclaration,
		nil,
		nil,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidIndexingError{}, errs[0])
}

func TestCheckInvalidDictionaryIndexingAssignmentWithType(t *testing.T) {

	_, err := ParseAndCheckWithExtra(t,
		`
          fun test() {
              let stuff = {"a": 1}
              stuff[String?] = "xyz"
          }
	    `,
		storageValueDeclaration,
		nil,
		nil,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidIndexingError{}, errs[0])
}

func TestCheckConstantSizedArrayDeclaration(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          let x: [Int; 3] = [1, 2, 3]
      }
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidConstantSizedArrayDeclarationCountMismatch(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          let x: [Int; 2] = [1, 2, 3]
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}
