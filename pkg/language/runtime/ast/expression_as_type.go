package ast

// ExpressionAsType attempts to convert an expression to a type.
// Some expressions can be considered both an expression and a type
//
func ExpressionAsType(expression Expression) Type {
	switch expression := expression.(type) {
	case *IdentifierExpression:
		return &NominalType{
			Identifier: expression.Identifier,
		}

	case *ArrayExpression:
		if len(expression.Values) != 1 {
			return nil
		}

		elementType := ExpressionAsType(expression.Values[0])
		if elementType == nil {
			return nil
		}

		return &VariableSizedType{
			Type: elementType,
			Range: Range{
				StartPos: expression.StartPos,
				EndPos:   expression.EndPos,
			},
		}

	case *DictionaryExpression:
		if len(expression.Entries) != 1 {
			return nil
		}

		entry := expression.Entries[0]

		keyType := ExpressionAsType(entry.Key)
		if keyType == nil {
			return nil
		}

		valueType := ExpressionAsType(entry.Value)
		if valueType == nil {
			return nil
		}

		return &DictionaryType{
			KeyType:   keyType,
			ValueType: valueType,
			Range: Range{
				StartPos: expression.StartPos,
				EndPos:   expression.EndPos,
			},
		}

	default:
		return nil
	}
}
