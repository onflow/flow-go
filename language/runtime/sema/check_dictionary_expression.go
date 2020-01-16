package sema

import "github.com/dapperlabs/flow-go/language/runtime/ast"

func (checker *Checker) VisitDictionaryExpression(expression *ast.DictionaryExpression) ast.Repr {

	// visit all entries, ensure key are all the same type,
	// and values are all the same type

	var keyType, valueType Type

	entryTypes := make([]DictionaryEntryType, len(expression.Entries))

	for i, entry := range expression.Entries {
		// NOTE: important to check move after each type check,
		// not combined after both type checks!

		entryKeyType := entry.Key.Accept(checker).(Type)
		checker.checkVariableMove(entry.Key)
		checker.checkResourceMoveOperation(entry.Key, entryKeyType)

		entryValueType := entry.Value.Accept(checker).(Type)
		checker.checkVariableMove(entry.Value)
		checker.checkResourceMoveOperation(entry.Value, entryValueType)

		entryTypes[i] = DictionaryEntryType{
			KeyType:   entryKeyType,
			ValueType: entryValueType,
		}

		// infer key type from first entry's key
		// TODO: find common super type?
		if keyType == nil {
			keyType = entryKeyType
		} else if !entryKeyType.IsInvalidType() &&
			!IsSubType(entryKeyType, keyType) {

			checker.report(
				&TypeMismatchError{
					ExpectedType: keyType,
					ActualType:   entryKeyType,
					Range:        ast.NewRangeFromPositioned(entry.Key),
				},
			)
		}

		// infer value type from first entry's value
		// TODO: find common super type?
		if valueType == nil {
			valueType = entryValueType
		} else if !entryValueType.IsInvalidType() &&
			!IsSubType(entryValueType, valueType) {

			checker.report(
				&TypeMismatchError{
					ExpectedType: valueType,
					ActualType:   entryValueType,
					Range:        ast.NewRangeFromPositioned(entry.Value),
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

	if !IsValidDictionaryKeyType(keyType) {
		checker.report(
			&InvalidDictionaryKeyTypeError{
				Type:  keyType,
				Range: ast.NewRangeFromPositioned(expression),
			},
		)
	}

	dictionaryType := &DictionaryType{
		KeyType:   keyType,
		ValueType: valueType,
	}

	checker.Elaboration.DictionaryExpressionEntryTypes[expression] = entryTypes
	checker.Elaboration.DictionaryExpressionType[expression] = dictionaryType

	return dictionaryType
}

func IsValidDictionaryKeyType(keyType Type) bool {
	// TODO: implement support for more built-in types here and in interpreter
	switch keyType.(type) {
	case *NeverType, *StringType, *BoolType:
		return true
	default:
		if IsSubType(keyType, &IntegerType{}) {
			return true
		}

		return false
	}
}
