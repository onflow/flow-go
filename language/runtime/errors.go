package runtime

import (
	"github.com/dapperlabs/bamboo-node/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/language/runtime/interpreter"
	"fmt"
	"github.com/logrusorgru/aurora"
	"strconv"
	"strings"
)

func colorizeError(message string) string {
	return aurora.Colorize(message, aurora.RedFg|aurora.BrightFg|aurora.BoldFm).String()
}

func colorizeMessage(message string) string {
	return aurora.Bold(message).String()
}

func colorizeMeta(meta string) string {
	return aurora.Blue(meta).String()
}

const errorPrefix = "error"
const errorArrow = "--> "

func FormatErrorMessage(message string, useColor bool) string {
	// prepare prefix
	formattedErrorPrefix := errorPrefix
	if useColor {
		formattedErrorPrefix = colorizeError(errorPrefix)
	}

	// prepare message
	message = ": " + message
	if useColor {
		message = colorizeMessage(message)
	}

	return formattedErrorPrefix + message + "\n"
}

func PrettyPrintError(err error, filename string, code string, useColor bool) string {
	var builder strings.Builder

	builder.WriteString(FormatErrorMessage(err.Error(), useColor))

	// get position, if any
	var startPosition, endPosition *ast.Position
	lineNumberString := ""
	lineNumberLength := 0
	if positioned, hasPosition := err.(ast.HasPosition); hasPosition {
		startPosition = positioned.StartPosition()
		endPosition = positioned.EndPosition()
		plainLineNumberString := strconv.Itoa(startPosition.Line)
		lineNumberLength = len(plainLineNumberString)

		// prepare line number string
		lineNumberString = plainLineNumberString + " | "
		if useColor {
			lineNumberString = colorizeMeta(lineNumberString)
		}
	}

	// write arrow, filename, and position (if any)
	writeErrorLocation(&builder, useColor, filename, lineNumberLength, startPosition)

	// code, if position
	if startPosition != nil {
		// prepare empty line numbers
		emptyLineNumbers := strings.Repeat(" ", lineNumberLength+1) + "|"
		if useColor {
			emptyLineNumbers = colorizeMeta(emptyLineNumbers)
		}

		// empty line
		builder.WriteString(emptyLineNumbers)
		builder.WriteString("\n")

		// line number
		builder.WriteString(lineNumberString)

		// code line
		lines := strings.Split(code, "\n")
		line := lines[startPosition.Line-1]
		builder.WriteString(line)
		builder.WriteString("\n")

		// indicator line
		builder.WriteString(emptyLineNumbers)

		for i := 0; i <= startPosition.Column; i++ {
			builder.WriteString(" ")
		}

		columns := 1
		if endPosition != nil && endPosition.Line == startPosition.Line {
			columns = endPosition.Column - startPosition.Column + 1
		}

		carets := strings.Repeat("^", columns)
		if useColor {
			carets = colorizeError(carets)
		}
		builder.WriteString(carets)

		if secondaryError, ok := err.(interpreter.SecondaryError); ok {
			builder.WriteString(" ")
			secondaryError := secondaryError.SecondaryError()
			if useColor {
				secondaryError = colorizeError(secondaryError)
			}
			builder.WriteString(secondaryError)
		}

		builder.WriteString("\n")
	}

	return builder.String()
}

func writeErrorLocation(
	builder *strings.Builder,
	useColor bool,
	filename string,
	lineNumberLength int,
	startPosition *ast.Position,
) {
	// write spaces before arrow
	for i := 0; i < lineNumberLength; i++ {
		builder.WriteString(" ")
	}

	// write arrow
	if useColor {
		builder.WriteString(colorizeMeta(errorArrow))
	} else {
		builder.WriteString(errorArrow)
	}

	// write filename
	builder.WriteString(filename)

	// write position (line and column)
	if startPosition != nil {
		_, err := fmt.Fprintf(builder, ":%d:%d", startPosition.Line, startPosition.Column)
		if err != nil {
			panic(err)
		}
	}
	builder.WriteString("\n")
}
