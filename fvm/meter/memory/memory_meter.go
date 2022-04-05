package memory

import (
	"github.com/onflow/cadence/runtime/common"
)

var memory_weights = [...]uint64{
	/* MemoryKindUnknown */ 0,
	/* MemoryKindBool */ 0,
	/* MemoryKindAddress */ 0,
	/* MemoryKindString */ 0,
	/* MemoryKindCharacter */ 0,
	/* MemoryKindMetaType */ 0,
	/* MemoryKindNumber */ 0,
	/* MemoryKindArray */ 0,
	/* MemoryKindDictionary */ 0,
	/* MemoryKindComposite */ 0,
	/* MemoryKindOptional */ 0,
	/* MemoryKindNil */ 0,
	/* MemoryKindVoid */ 0,
	/* MemoryKindTypeValue */ 0,
	/* MemoryKindPathValue */ 0,
	/* MemoryKindCapabilityValue */ 0,
	/* MemoryKindLinkValue */ 0,
	/* MemoryKindStorageReferenceValue */ 0,
	/* MemoryKindEphemeralReferenceValue */ 0,
	/* MemoryKindInterpretedFunction */ 0,
	/* MemoryKindHostFunction */ 0,
	/* MemoryKindBoundFunction */ 0,
	/* MemoryKindBigInt */ 0,

	/* MemoryKindRawString */ 0,
	/* MemoryKindAddressLocation */ 0,
	/* MemoryKindBytes */ 0,
	/* MemoryKindVariable */ 0,

	/* MemoryKindTokenIdentifier */ 0,
	/* MemoryKindTokenComment */ 0,
	/* MemoryKindTokenNumericLiteral */ 0,
	/* MemoryKindTokenSyntax */ 0,
}

const MaximumMemoryAllowance uint64 = 0

func _() {
	// A compiler error signifies that we have not accounted for all memory kinds
	var x [1]struct{}
	_ = x[int(common.MemoryKindTokenSyntax)+1-len(memory_weights)]
}

type OutOfMemoryError struct {
}

func (e *OutOfMemoryError) Error() string {
	return "Ran out of memory"

}

type MemoryMeter struct {
	totalMemoryUsed uint64
}

func NewMemoryMeter() MemoryMeter {
	return MemoryMeter{
		totalMemoryUsed: 0,
	}
}

func (m MemoryMeter) ConsumeMemory(usage common.MemoryUsage) error {
	consumed := usage.Amount * memory_weights[usage.Kind]
	m.totalMemoryUsed = m.totalMemoryUsed + consumed
	if m.totalMemoryUsed > MaximumMemoryAllowance {
		return &OutOfMemoryError{}
	}
	return nil
}
