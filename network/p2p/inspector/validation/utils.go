package validation

type duplicateStrTracker map[string]struct{}

func (d duplicateStrTracker) set(s string) {
	d[s] = struct{}{}
}

func (d duplicateStrTracker) isDuplicate(s string) bool {
	_, ok := d[s]
	return ok
}
