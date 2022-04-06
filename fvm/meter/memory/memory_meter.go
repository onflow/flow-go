package memory

import (
	"runtime"

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

	/* MemoryKindIdentifier */ 0,
	/* MemoryKindArgument */ 0,
	/* MemoryKindBlock */ 0,

	/* MemoryKindFunctionDeclaration */ 0,
	/* MemoryKindCompositeDeclaration */ 0,
	/* MemoryKindInterfaceDeclaration */ 0,
	/* MemoryKindEnumCaseDeclaration */ 0,
	/* MemoryKindFieldDeclaration */ 0,
	/* MemoryKindTransactionDeclaration */ 0,
	/* MemoryKindImportDeclaration */ 0,
	/* MemoryKindVariableDeclaration */ 0,
	/* MemoryKindSpecialFunctionDeclaration */ 0,
	/* MemoryKindPragmaDeclaration */ 0,

	/* MemoryKindAssignmentStatement */ 0,
	/* MemoryKindBreakStatement */ 0,
	/* MemoryKindContinueStatement */ 0,
	/* MemoryKindEmitStatement */ 0,
	/* MemoryKindExpressionStatement */ 0,
	/* MemoryKindForStatement */ 0,
	/* MemoryKindIfStatement */ 0,
	/* MemoryKindReturnStatement */ 0,
	/* MemoryKindSwapStatement */ 0,
	/* MemoryKindSwitchStatement */ 0,
	/* MemoryKindWhileStatement */ 0,

	/* MemoryKindBooleanExpression */ 0,
	/* MemoryKindNilExpression */ 0,
	/* MemoryKindStringExpression */ 0,
	/* MemoryKindIntegerExpression */ 0,
	/* MemoryKindFixedPointExpression */ 0,
	/* MemoryKindArrayExpression */ 0,
	/* MemoryKindDictionaryExpression */ 0,
	/* MemoryKindIdentifierExpression */ 0,
	/* MemoryKindInvocationExpression */ 0,
	/* MemoryKindMemberExpression */ 0,
	/* MemoryKindIndexExpression */ 0,
	/* MemoryKindConditionalExpression */ 0,
	/* MemoryKindUnaryExpression */ 0,
	/* MemoryKindBinaryExpression */ 0,
	/* MemoryKindFunctionExpression */ 0,
	/* MemoryKindCastingExpression */ 0,
	/* MemoryKindCreateExpression */ 0,
	/* MemoryKindDestroyExpression */ 0,
	/* MemoryKindReferenceExpression */ 0,
	/* MemoryKindForceExpression */ 0,
	/* MemoryKindPathExpression */ 0,
}

const MaximumMemoryAllowance uint64 = 0

func _() {
	// A compiler error signifies that we have not accounted for all memory kinds
	var x [1]struct{}
	_ = x[int(common.MemoryKindLast)-len(memory_weights)]
}

type OutOfMemoryError struct {
}

func (e *OutOfMemoryError) Error() string {
	return "Ran out of memory"

}

type MemoryMeter struct {
	initialMemoryUsage uint64
	totalMemoryUsed    uint64
}

func NewMemoryMeter() MemoryMeter {
	var m runtime.MemStats
	return MemoryMeter{
		initialMemoryUsage: m.TotalAlloc,
		totalMemoryUsed:    0,
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
