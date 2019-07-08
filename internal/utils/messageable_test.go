package utils

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dapperlabs/bamboo-node/internal/types"
)

func TestRegister(t *testing.T) {
	gomega := NewWithT(t)

	message := types.MockRegisterMessage()
	register := types.MockRegister()

	actualMessage := RegisterToMessage(register)
	actualRegister := MessageToRegister(message)

	gomega.Expect(actualMessage).To(Equal(message))
	gomega.Expect(actualRegister).To(Equal(register))
	gomega.Expect(RegisterToMessage(actualRegister)).To(Equal(message))
	gomega.Expect(MessageToRegister(actualMessage)).To(Equal(register))
}

func TestIntermediateRegisters(t *testing.T) {
	gomega := NewWithT(t)

	message := types.MockIntermediateRegistersMessage()
	intermediateRegisters := types.MockIntermediateRegisters()

	actualMessage := IntermediateRegistersToMessage(intermediateRegisters)
	actualIntermediateRegisters := MessageToIntermediateRegisters(message)

	gomega.Expect(actualMessage).To(Equal(message))
	gomega.Expect(actualIntermediateRegisters).To(Equal(intermediateRegisters))
	gomega.Expect(IntermediateRegistersToMessage(actualIntermediateRegisters)).To(Equal(message))
	gomega.Expect(MessageToIntermediateRegisters(actualMessage)).To(Equal(intermediateRegisters))
}
