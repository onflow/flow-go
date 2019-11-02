package runtime

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/logrusorgru/aurora"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
)

func colorizeError(message string) string {
	return aurora.Colorize(message, aurora.RedFg|aurora.BrightFg|aurora.BoldFm).String()
}

func colorizeNote(message string) string {
	return aurora.Colorize(message, aurora.CyanFg|aurora.BoldFm).String()
}

func colorizeMessage(message string) string {
	return aurora.Bold(message).String()
}

func colorizeMeta(meta string) string {
	return aurora.Blue(meta).String()
}

const errorPrefix = "error"
const excerptArrow = "--> "
const excerptDots = "... "

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

type excerpt struct {
	startPos *ast.Position
	endPos   *ast.Position
	message  string
	isError  bool
}

func newExcerpt(obj interface{}, message string, isError bool) *excerpt {
	excerpt := &excerpt{
		message: message,
		isError: isError,
	}
	if positioned, hasPosition := obj.(ast.HasPosition); hasPosition {
		startPos := positioned.StartPosition()
		excerpt.startPos = &startPos

		endPos := positioned.EndPosition()
		excerpt.endPos = &endPos
	}
	return excerpt
}

func PrettyPrintError(err error, filename string, code string, useColor bool) string {
	var builder strings.Builder

	builder.WriteString(FormatErrorMessage(err.Error(), useColor))

	message := ""
	if secondaryError, ok := err.(errors.SecondaryError); ok {
		message = secondaryError.SecondaryError()
	}

	excerpts := []*excerpt{
		newExcerpt(err, message, true),
	}

	if errorNotes, ok := err.(errors.ErrorNotes); ok {
		for _, errorNote := range errorNotes.ErrorNotes() {
			excerpts = append(excerpts,
				newExcerpt(errorNote, errorNote.Message(), false),
			)
		}
	}

	sortExcerpts(excerpts)

	writeCodeExcerpts(&builder, excerpts, filename, code, useColor)

	return builder.String()
}

func sortExcerpts(excerpts []*excerpt) {
	sort.Slice(excerpts, func(i, j int) bool {
		first := excerpts[i]
		second := excerpts[j]
		if first.startPos == nil || second.startPos == nil {
			return false
		}
		if first.startPos.Line < second.startPos.Line {
			return true
		}
		if first.startPos.Line > second.startPos.Line {
			return false
		}
		if first.startPos.Column < second.startPos.Column {
			return true
		}
		return false
	})
}

func writeCodeExcerpts(
	builder *strings.Builder,
	excerpts []*excerpt,
	filename string,
	code string,
	useColor bool,
) {
	var lastLineNumber int

	for i, excerpt := range excerpts {

		lineNumberString := ""
		lineNumberLength := 0
		if excerpt.startPos != nil {

			plainLineNumberString := strconv.Itoa(excerpt.startPos.Line)
			lineNumberLength = len(plainLineNumberString)

			// prepare line number string
			lineNumberString = plainLineNumberString + " | "
			if useColor {
				lineNumberString = colorizeMeta(lineNumberString)
			}
		}

		// write arrow, filename, and position (if any)
		if i == 0 {
			writeCodeExcerptLocation(builder, useColor, filename, lineNumberLength, excerpt.startPos)
		}

		// code, if position
		if excerpt.startPos != nil {

			if i > 0 && lastLineNumber != 0 && excerpt.startPos.Line-1 > lastLineNumber {
				writeCodeExcerptContinuation(builder, lineNumberLength, useColor)
			}
			lastLineNumber = excerpt.startPos.Line

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
			line := lines[excerpt.startPos.Line-1]
			builder.WriteString(line)
			builder.WriteString("\n")

			// indicator line
			builder.WriteString(emptyLineNumbers)

			for i := 0; i <= excerpt.startPos.Column; i++ {
				builder.WriteString(" ")
			}

			columns := 1
			if excerpt.endPos != nil && excerpt.endPos.Line == excerpt.startPos.Line {
				columns = excerpt.endPos.Column - excerpt.startPos.Column + 1
			}

			indicator := "-"
			if excerpt.isError {
				indicator = "^"
			}

			indicators := strings.Repeat(indicator, columns)
			if useColor {
				if excerpt.isError {
					indicators = colorizeError(indicators)
				} else {
					indicators = colorizeNote(indicators)
				}
			}
			builder.WriteString(indicators)

			if excerpt.message != "" {
				message := excerpt.message
				builder.WriteString(" ")
				if useColor {
					if excerpt.isError {
						message = colorizeError(message)
					} else {
						message = colorizeNote(message)
					}
				}
				builder.WriteString(message)
			}

			builder.WriteString("\n")
		} else {
			lastLineNumber = 0
		}
	}
}

func writeCodeExcerptLocation(
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
		builder.WriteString(colorizeMeta(excerptArrow))
	} else {
		builder.WriteString(excerptArrow)
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

func writeCodeExcerptContinuation(builder *strings.Builder, lineNumberLength int, useColor bool) {
	// write spaces before dots
	for i := 0; i < lineNumberLength; i++ {
		builder.WriteString(" ")
	}

	// write dots
	dots := excerptDots
	if useColor {
		dots = colorizeMeta(dots)
	}
	builder.WriteString(dots)

	builder.WriteString("\n")
}
