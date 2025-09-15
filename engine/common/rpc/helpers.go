package rpc

// CheckScriptSize returns true if the combined size (in bytes) of the script and arguments is less
// than or equal to the max size.
func CheckScriptSize(script []byte, arguments [][]byte, maxSize uint) bool {
	currentSize := len(script)
	if currentSize > int(maxSize) {
		return false
	}

	for _, arg := range arguments {
		currentSize += len(arg)
		if currentSize > int(maxSize) {
			return false
		}
	}

	return true
}
