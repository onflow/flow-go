package ast

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestProgram_ResolveImports(t *testing.T) {
	RegisterTestingT(t)

	makeImportingProgram := func(imported string) *Program {
		return &Program{
			Declarations: []Declaration{
				&ImportDeclaration{
					Location: StringImportLocation(imported),
				},
			},
		}
	}

	a := makeImportingProgram("b")
	b := makeImportingProgram("c")
	c := &Program{}

	err := a.ResolveImports(func(location ImportLocation) *Program {
		switch location {
		case StringImportLocation("b"):
			return b
		case StringImportLocation("c"):
			return c
		default:
			t.Fatalf("Tried to resolve unknown import location: %s", location)
			return nil
		}
	})

	Expect(err).To(Not(HaveOccurred()))

	Expect(a.Imports()[StringImportLocation("b")]).
		To(BeIdenticalTo(b))

	Expect(b.Imports()[StringImportLocation("c")]).
		To(BeIdenticalTo(c))
}

func TestProgram_ResolveImportsCycle(t *testing.T) {
	RegisterTestingT(t)

	makeImportingProgram := func(imported string) *Program {
		return &Program{
			Declarations: []Declaration{
				&ImportDeclaration{
					Location: StringImportLocation(imported),
				},
			},
		}
	}

	a := makeImportingProgram("b")
	b := makeImportingProgram("c")
	c := makeImportingProgram("a")

	err := a.ResolveImports(func(location ImportLocation) *Program {
		switch location {
		case StringImportLocation("a"):
			return a
		case StringImportLocation("b"):
			return b
		case StringImportLocation("c"):
			return c
		default:
			t.Fatalf("Tried to resolve unknown import location: %s", location)
			return nil
		}
	})

	Expect(err).
		To(Equal(CyclicImportsError{
			Location: StringImportLocation("a"),
		}))
}
