package rpc

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
