package sema

import "github.com/dapperlabs/flow-go/pkg/language/runtime/ast"

func (checker *Checker) VisitDictionaryExpression(expression *ast.DictionaryExpression) ast.Repr {

	// visit all entries, ensure key are all the same type,
	// and values are all the same type

	var keyType, valueType Type

	for _, entry := range expression.Entries {
		entryKeyType := entry.Key.Accept(checker).(Type)
		entryValueType := entry.Value.Accept(checker).(Type)

		// infer key type from first entry's key
		// TODO: find common super type?
		if keyType == nil {
			keyType = entryKeyType
		} else if !IsSubType(entryKeyType, keyType) {
			checker.report(
				&TypeMismatchError{
					ExpectedType: keyType,
					ActualType:   entryKeyType,
					StartPos:     entry.Key.StartPosition(),
					EndPos:       entry.Key.EndPosition(),
				},
			)
		}

		// infer value type from first entry's value
		// TODO: find common super type?
		if valueType == nil {
			valueType = entryValueType
		} else if !IsSubType(entryValueType, valueType) {
			checker.report(
				&TypeMismatchError{
					ExpectedType: valueType,
					ActualType:   entryValueType,
					StartPos:     entry.Value.StartPosition(),
					EndPos:       entry.Value.EndPosition(),
				},
			)
		}
	}

	if keyType == nil {
		keyType = &NeverType{}
	}
	if valueType == nil {
		valueType = &NeverType{}
	}

	return &DictionaryType{
		KeyType:   keyType,
		ValueType: valueType,
	}
}
