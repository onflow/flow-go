package sema

type ReturnInfo struct {
	MaybeReturned      bool
	DefinitelyReturned bool
}

func (ri *ReturnInfo) MergeBranches(thenReturnInfo *ReturnInfo, elseReturnInfo *ReturnInfo) {
	ri.MaybeReturned = ri.MaybeReturned ||
		thenReturnInfo.MaybeReturned ||
		elseReturnInfo.MaybeReturned

	ri.DefinitelyReturned = ri.DefinitelyReturned ||
		(thenReturnInfo.DefinitelyReturned &&
			elseReturnInfo.DefinitelyReturned)
}

func (ri *ReturnInfo) Clone() *ReturnInfo {
	result := &ReturnInfo{}
	*result = *ri
	return result
}
