package sema

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/parser"
)

// testAssignment parses a composite declaration and performs definite assignment
// analysis on the first initializer defined.
//
// This function returns a list of all fields that are not definitely assigned
// in the initializer, as well as any errors that occurred due to unassigned usages.
func testAssignment(body string) ([]*ast.FieldDeclaration, []error) {
	program, _ := parser.ParseProgram(body)

	structDeclaration := program.CompositeDeclarations()[0]

	fields := structDeclaration.Members.Fields
	initializer := structDeclaration.Members.Initializers[0]

	return CheckFieldAssignments(fields, initializer.FunctionBlock)
}

func TestAssignmentInEmptyInitializer(t *testing.T) {
	RegisterTestingT(t)

	body := `
		struct Test {
			pub(set) var foo: Int
			pub(set) var bar: Int

			init(foo: Int) {}
		}
	`

	unassigned, errors := testAssignment(body)

	Expect(unassigned).To((HaveLen(2)))
	Expect(errors).To((HaveLen(0)))
}

func TestAssignmentFromArgument(t *testing.T) {
	RegisterTestingT(t)

	body := `
		struct Test {
			pub(set) var foo: Int

			init(foo: Int) {
				self.foo = foo
			}
		}
	`

	unassigned, errors := testAssignment(body)

	Expect(unassigned).To((HaveLen(0)))
	Expect(errors).To((HaveLen(0)))
}

func TestAssignmentInIfStatement(t *testing.T) {
	t.Run("ValidIfStatement", func(t *testing.T) {
		RegisterTestingT(t)

		body := `
			struct Test {
				pub(set) var foo: Int
	
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

		Expect(unassigned).To((HaveLen(0)))
		Expect(errors).To((HaveLen(0)))
	})

	t.Run("InvalidIfStatement", func(t *testing.T) {
		RegisterTestingT(t)

		body := `
			struct Test {
				pub(set) var foo: Int
	
				init() {
					if 1 > 2 {
						self.foo = 1
					}
				}
			}
		`

		unassigned, errors := testAssignment(body)

		Expect(unassigned).To((HaveLen(1)))
		Expect(errors).To((HaveLen(0)))
	})
}

func TestAssignmentInWhileStatement(t *testing.T) {
	RegisterTestingT(t)

	body := `
			struct Test {
				pub(set) var foo: Int
	
				init() {
					while 1 < 2 {
						self.foo = 1	
					}
				}
			}
		`

	unassigned, errors := testAssignment(body)

	Expect(unassigned).To((HaveLen(1)))
	Expect(errors).To((HaveLen(0)))
}

func TestAssignmentFromField(t *testing.T) {
	t.Run("FromInitializedField", func(t *testing.T) {
		RegisterTestingT(t)

		body := `
			struct Test {
				pub(set) var foo: Int
				pub(set) var bar: Int

				init() {
					self.foo = 1
					self.bar = self.foo + 1
				}
			}
		`

		unassigned, errors := testAssignment(body)

		Expect(unassigned).To((HaveLen(0)))
		Expect(errors).To((HaveLen(0)))
	})

	t.Run("FromUninitializedField", func(t *testing.T) {
		RegisterTestingT(t)

		body := `
			struct Test {
				pub(set) var foo: Int
				pub(set) var bar: Int

				init() {
					self.bar = self.foo + 1
				}
			}
		`

		unassigned, errors := testAssignment(body)

		Expect(unassigned).To((HaveLen(1)))
		Expect(errors).To((HaveLen(1)))
	})
}

func TestAssignmentUsages(t *testing.T) {
	t.Run("InitializedUsage", func(t *testing.T) {
		RegisterTestingT(t)

		body := `
			fun myFunc(x: Int) {}

			struct Test {
				pub(set) var foo: Int

				init() {
					self.foo = 1
					myFunc(self.foo)
				}
			}
		`

		unassigned, errors := testAssignment(body)

		Expect(unassigned).To((HaveLen(0)))
		Expect(errors).To((HaveLen(0)))
	})

	t.Run("UninitializedUsage", func(t *testing.T) {
		RegisterTestingT(t)

		body := `
			fun myFunc(x: Int) {}

			struct Test {
				pub(set) var foo: Int

				init() {
					myFunc(self.foo)
					self.foo = 1
				}
			}
		`

		unassigned, errors := testAssignment(body)

		Expect(unassigned).To((HaveLen(0)))
		Expect(errors).To((HaveLen(1)))
	})

	t.Run("IfStatementUsage", func(t *testing.T) {
		RegisterTestingT(t)

		body := `
			struct Test {
				pub(set) var foo: Int

				init() {
					if self.foo > 0 {
						
					}

					self.foo = 1
				}
			}
		`

		unassigned, errors := testAssignment(body)

		Expect(unassigned).To((HaveLen(0)))
		Expect(errors).To((HaveLen(1)))
	})

	t.Run("ArrayLiteralUsage", func(t *testing.T) {
		RegisterTestingT(t)

		body := `
			struct Test {
				pub(set) var foo: Int

				init() {	
					var x = [self.foo]

					self.foo = 1
				}
			}
		`

		unassigned, errors := testAssignment(body)

		Expect(unassigned).To((HaveLen(0)))
		Expect(errors).To((HaveLen(1)))
	})

	t.Run("BinaryOperationUsage", func(t *testing.T) {
		RegisterTestingT(t)

		body := `
			struct Test {
				pub(set) var foo: Int

				init() {	
					var x = 4 + self.foo

					self.foo = 1
				}
			}
		`

		unassigned, errors := testAssignment(body)

		Expect(unassigned).To((HaveLen(0)))
		Expect(errors).To((HaveLen(1)))
	})

	t.Run("ComplexUsages", func(t *testing.T) {
		RegisterTestingT(t)

		body := `
			struct Test {
				pub(set) var a: Int
				pub(set) var b: Int
				pub(set) var c: Int

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

		Expect(unassigned).To((HaveLen(0)))
		Expect(errors).To((HaveLen(0)))
	})
}
