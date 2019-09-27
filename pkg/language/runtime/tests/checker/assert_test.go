package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/stdlib"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCheckAssertWithoutMessage(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheckWithExtra(
		`
            fun test() {
                assert(1 == 2)
            }
        `,
		stdlib.StandardLibraryFunctions{
			stdlib.AssertFunction,
		}.ToValueDeclarations(),
		nil,
		nil,
	)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckAssertWithMessage(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheckWithExtra(
		`
            fun test() {
                assert(1 == 2, message: "test message")
            }
        `,
		stdlib.StandardLibraryFunctions{
			stdlib.AssertFunction,
		}.ToValueDeclarations(),
		nil,
		nil,
	)

	Expect(err).
		To(Not(HaveOccurred()))
}
