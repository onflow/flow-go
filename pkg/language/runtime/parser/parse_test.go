package parser

import (
	. "github.com/onsi/gomega"
	"testing"
)

func TestParseIncomplete(t *testing.T) {
	RegisterTestingT(t)

	program, inputIsComplete, err := ParseProgram("struct X")

	Expect(program).
		To(BeNil())

	Expect(inputIsComplete).
		To(BeFalse())

	Expect(err).
		To(Not(BeNil()))
}
