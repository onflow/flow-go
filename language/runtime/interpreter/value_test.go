package interpreter

import (
	. "github.com/onsi/gomega"
	"testing"
)

func TestToExpression(t *testing.T) {
	RegisterTestingT(t)

	_, err := ToValue(1)
	Expect(err).Should(HaveOccurred())
	Expect(ToValue(int8(1))).To(Equal(Int8Value(1)))
	Expect(ToValue(int16(2))).To(Equal(Int16Value(2)))
	Expect(ToValue(int32(3))).To(Equal(Int32Value(3)))
	Expect(ToValue(int64(4))).To(Equal(Int64Value(4)))
	Expect(ToValue(uint8(1))).To(Equal(UInt8Value(1)))
	Expect(ToValue(uint16(2))).To(Equal(UInt16Value(2)))
	Expect(ToValue(uint32(3))).To(Equal(UInt32Value(3)))
	Expect(ToValue(uint64(4))).To(Equal(UInt64Value(4)))
	Expect(ToValue(true)).To(Equal(BoolValue(true)))
	Expect(ToValue(false)).To(Equal(BoolValue(false)))
}
