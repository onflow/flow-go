package utils

import (
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
)

type OccurrenceMatcher struct {
	StartPos        sema.Position
	EndPos          sema.Position
	OriginStartPos  *sema.Position
	OriginEndPos    *sema.Position
	DeclarationKind common.DeclarationKind
}

func (matcher *OccurrenceMatcher) Match(actual interface{}) bool {
	occurrence, ok := actual.(sema.Occurrence)
	if !ok {
		return false
	}

	if occurrence.StartPos != matcher.StartPos {
		return false
	}

	if occurrence.EndPos != matcher.EndPos {
		return false
	}

	if occurrence.Origin.DeclarationKind != matcher.DeclarationKind {
		return false
	}

	if occurrence.Origin.StartPos != nil {
		if occurrence.Origin.StartPos.Line != matcher.OriginStartPos.Line ||
			occurrence.Origin.StartPos.Column != matcher.OriginStartPos.Column {
			return false
		}
	}

	if occurrence.Origin.EndPos != nil {
		if occurrence.Origin.EndPos.Line != matcher.OriginEndPos.Line ||
			occurrence.Origin.EndPos.Column != matcher.OriginEndPos.Column {
			return false
		}
	}

	return true
}
