package ast

import (
	. "github.com/onsi/gomega"
	"testing"
)

func TestToExpression(t *testing.T) {
	RegisterTestingT(t)

	Expect(func() { ToExpression(1) }).Should(Panic())
	Expect(ToExpression(int8(1))).To(Equal(Int8Expression(1)))
	Expect(ToExpression(int16(2))).To(Equal(Int16Expression(2)))
	Expect(ToExpression(int32(3))).To(Equal(Int32Expression(3)))
	Expect(ToExpression(int64(4))).To(Equal(Int64Expression(4)))
	Expect(ToExpression(uint8(1))).To(Equal(UInt8Expression(1)))
	Expect(ToExpression(uint16(2))).To(Equal(UInt16Expression(2)))
	Expect(ToExpression(uint32(3))).To(Equal(UInt32Expression(3)))
	Expect(ToExpression(uint64(4))).To(Equal(UInt64Expression(4)))
	Expect(ToExpression(true)).To(Equal(BoolExpression(true)))
	Expect(ToExpression(false)).To(Equal(BoolExpression(false)))
}
