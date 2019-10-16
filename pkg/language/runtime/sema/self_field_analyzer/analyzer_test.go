package self_field_analyzer

import (
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/parser"
)

// testAssignment parses a composite declaration and performs definite assignment
// analysis on the first initializer defined.
//
// This function returns a list of all fields that are not definitely assigned
// in the initializer, as well as any errors that occurred due to unassigned usages.
func testAssignment(body string) ([]*ast.FieldDeclaration, []error) {
	program, _, err := parser.ParseProgram(body)
	if err != nil {
		panic(err)
	}

	structDeclaration := program.CompositeDeclarations()[0]

	fields := structDeclaration.Members.Fields
	initializer := structDeclaration.Members.SpecialFunctions[0]

	return CheckSelfFieldInitializations(fields, initializer.FunctionBlock)
}

func TestAssignmentInEmptyInitializer(t *testing.T) {

	body := `
		struct Test {
			var foo: Int
			var bar: Int

			init(foo: Int) {}
		}
	`

	unassigned, errors := testAssignment(body)

	assert.Len(t, unassigned, 2)
	assert.Len(t, errors, 0)
}

func TestAssignmentFromArgument(t *testing.T) {

	body := `
		struct Test {
			var foo: Int

			init(foo: Int) {
				self.foo = foo
			}
		}
	`

	unassigned, errors := testAssignment(body)

	assert.Len(t, unassigned, 0)
	assert.Len(t, errors, 0)
}

func TestAssignmentInIfStatement(t *testing.T) {
	t.Run("ValidIfStatement", func(t *testing.T) {

		body := `
			struct Test {
				var foo: Int
	
				init() {	
					if 1 > 2 {
						self.foo = 1
					} else {
						self.foo = 2
					}
				}
			}
		`

		unassigned, errors := testAssignment(body)

		assert.Len(t, unassigned, 0)
		assert.Len(t, errors, 0)
	})

	t.Run("InvalidIfStatement", func(t *testing.T) {

		body := `
			struct Test {
				var foo: Int
	
				init() {
					if 1 > 2 {
						self.foo = 1
					}
				}
			}
		`

		unassigned, errors := testAssignment(body)

		assert.Len(t, unassigned, 1)
		assert.Len(t, errors, 0)
	})
}

func TestAssignmentInWhileStatement(t *testing.T) {

	body := `
			struct Test {
				var foo: Int
	
				init() {
					while 1 < 2 {
						self.foo = 1	
					}
				}
			}
		`

	unassigned, errors := testAssignment(body)

	assert.Len(t, unassigned, 1)
	assert.Len(t, errors, 0)
}

func TestAssignmentFromField(t *testing.T) {
	t.Run("FromInitializedField", func(t *testing.T) {

		body := `
			struct Test {
				var foo: Int
				var bar: Int

				init() {
					self.foo = 1
					self.bar = self.foo + 1
				}
			}
		`

		unassigned, errors := testAssignment(body)

		assert.Len(t, unassigned, 0)
		assert.Len(t, errors, 0)
	})

	t.Run("FromUninitializedField", func(t *testing.T) {

		body := `
			struct Test {
				var foo: Int
				var bar: Int

				init() {
					self.bar = self.foo + 1
				}
			}
		`

		unassigned, errors := testAssignment(body)

		assert.Len(t, unassigned, 1)
		assert.Len(t, errors, 1)
	})
}

func TestAssignmentUsages(t *testing.T) {
	t.Run("InitializedUsage", func(t *testing.T) {

		body := `
			fun myFunc(x: Int) {}

			struct Test {
				var foo: Int

				init() {
					self.foo = 1
					myFunc(self.foo)
				}
			}
		`

		unassigned, errors := testAssignment(body)

		assert.Len(t, unassigned, 0)
		assert.Len(t, errors, 0)
	})

	t.Run("UninitializedUsage", func(t *testing.T) {

		body := `
			fun myFunc(x: Int) {}

			struct Test {
				var foo: Int

				init() {
					myFunc(self.foo)
					self.foo = 1
				}
			}
		`

		unassigned, errors := testAssignment(body)

		assert.Len(t, unassigned, 0)
		assert.Len(t, errors, 1)
	})

	t.Run("IfStatementUsage", func(t *testing.T) {

		body := `
			struct Test {
				var foo: Int

				init() {
					if self.foo > 0 {
						
					}

					self.foo = 1
				}
			}
		`

		unassigned, errors := testAssignment(body)

		assert.Len(t, unassigned, 0)
		assert.Len(t, errors, 1)
	})

	t.Run("ArrayLiteralUsage", func(t *testing.T) {

		body := `
			struct Test {
				var foo: Int

				init() {	
					var x = [self.foo]

					self.foo = 1
				}
			}
		`

		unassigned, errors := testAssignment(body)

		assert.Len(t, unassigned, 0)
		assert.Len(t, errors, 1)
	})

	t.Run("BinaryOperationUsage", func(t *testing.T) {

		body := `
			struct Test {
				var foo: Int

				init() {	
					var x = 4 + self.foo

					self.foo = 1
				}
			}
		`

		unassigned, errors := testAssignment(body)

		assert.Len(t, unassigned, 0)
		assert.Len(t, errors, 1)
	})

	t.Run("ComplexUsages", func(t *testing.T) {

		body := `
			struct Test {
				var a: Int
				var b: Int
				var c: Int

				init(x: Int) {
					self.a = x
					
					if self.a < 4 {
						self.b = self.a + 2
					} else {
						self.b = 0
					}
					
					if self.a + self.b < 5 {
						self.c = self.a
					} else {
						self.c = self.b
					}
				}
			}
		`

		unassigned, errors := testAssignment(body)

		assert.Len(t, unassigned, 0)
		assert.Len(t, errors, 0)
	})
}

func TestAssignmentWithReturn(t *testing.T) {

	t.Run("Direct", func(t *testing.T) {

		body := `
			struct Test {
				var foo: Int

				init(foo: Int) {
					return
					self.foo = foo
				}
			}
		`

		unassigned, errors := testAssignment(body)

		assert.Len(t, unassigned, 1)
		assert.Len(t, errors, 0)
	})

	t.Run("InsideIf", func(t *testing.T) {

		body := `
			struct Test {
				var foo: Int

				init(foo: Int) {
					if false {
						return
					}
					self.foo = foo
				}
			}
		`

		unassigned, errors := testAssignment(body)

		assert.Len(t, unassigned, 1)
		assert.Len(t, errors, 0)
	})

	t.Run("InsideWhile", func(t *testing.T) {

		body := `
			struct Test {
				var foo: Int

				init(foo: Int) {
					while false {
						return
					}
					self.foo = foo
				}
			}
		`

		unassigned, errors := testAssignment(body)

		assert.Len(t, unassigned, 1)
		assert.Len(t, errors, 0)
	})
}
