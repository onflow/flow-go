package main

import (
	"fmt"
	"io"
)

var (
	indent  = []byte("\t")
	newline = []byte("\n")
)

type Chunk struct {
	indentLevel int
	format      string
	args        []any
}

func (chunk *Chunk) WriteTo(writer io.Writer) (int64, error) {
	total := int64(0)

	if chunk.format != "" {
		for i := 0; i < chunk.indentLevel; i++ {
			n, err := writer.Write(indent)
			total += int64(n)

			if err != nil {
				return total, err
			}
		}

		n, err := fmt.Fprintf(writer, chunk.format, chunk.args...)
		total += int64(n)

		if err != nil {
			return total, err
		}
	}

	n, err := writer.Write(newline)
	total += int64(n)
	if err != nil {
		return total, err
	}

	return total, nil
}

type FileContent struct {
	indentLevel int
	chunks      []io.WriterTo
}

func NewFileContent() *FileContent {
	return &FileContent{
		indentLevel: 0,
		chunks:      nil,
	}
}

func (content *FileContent) PushIndent() {
	content.indentLevel += 1
}

func (content *FileContent) PopIndent() {
	if content.indentLevel > 0 {
		content.indentLevel -= 1
	}
}

func (content *FileContent) Line(format string, args ...any) {
	content.chunks = append(
		content.chunks,
		&Chunk{content.indentLevel, format, args})
}

func (content *FileContent) Section(format string, args ...any) {
	content.chunks = append(content.chunks, &Chunk{0, format, args})
}

func (content *FileContent) WriteTo(output io.Writer) (int64, error) {
	total := int64(0)
	for _, chunk := range content.chunks {
		n, err := chunk.WriteTo(output)
		total += n

		if err != nil {
			return total, err
		}
	}

	return total, nil
}
