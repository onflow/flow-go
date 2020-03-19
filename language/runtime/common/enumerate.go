package common

import (
	"fmt"
	"strings"
)

func EnumerateWords(words []string, conjunction string) string {
	count := len(words)
	switch count {
	case 0:
		return ""

	case 1:
		return words[0]

	case 2:
		return fmt.Sprintf("%s %s %s", words[0], conjunction, words[1])

	default:
		lastIndex := count - 1
		commaSeparatedExceptLastWord := strings.Join(words[:lastIndex], ", ")
		lastWord := words[lastIndex]
		return fmt.Sprintf("%s, %s %s", commaSeparatedExceptLastWord, conjunction, lastWord)
	}
}
