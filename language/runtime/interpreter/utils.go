package interpreter

import (
	"strings"
)

func PadLeft(value string, separator rune, minLength uint) string {
	length := uint(len(value))
	if length >= minLength {
		return value
	}
	n := int(minLength - length)

	var builder strings.Builder
	builder.Grow(n)
	for i := 0; i < n; i++ {
		builder.WriteRune(separator)
	}
	builder.WriteString(value)
	return builder.String()
}
