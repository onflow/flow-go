package debug

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	otelTrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/onflow/flow-go/module/trace"
)

type InterestingCadenceSpanExporter struct {
	Spans []otelTrace.ReadOnlySpan
	Log   bool
}

var _ otelTrace.SpanExporter = &InterestingCadenceSpanExporter{}

var interestingSpanNamePrefixes = []trace.SpanName{
	trace.FVMCadenceTrace,
	trace.FVMEnvAllocateSlabIndex,
	trace.FVMEnvGetValue,
	trace.FVMEnvSetValue,
	trace.FVMEnvGetOrLoadProgram,
}

var uninterestingSpanNames = []trace.SpanName{
	// Only reported by interpreter at the moment, makes diffing harder
	trace.FVMCadenceTrace + ".import",
}

func (s *InterestingCadenceSpanExporter) ExportSpans(_ context.Context, spans []otelTrace.ReadOnlySpan) error {
	for _, span := range spans {
		name := span.Name()
		for _, prefix := range interestingSpanNamePrefixes {

			// Filter spans
			if strings.HasPrefix(name, string(prefix)) &&
				!slices.Contains(uninterestingSpanNames, trace.SpanName(name)) {

				s.Spans = append(s.Spans, span)

				if s.Log {
					err := writeSpan(os.Stderr, span)
					if err != nil {
						return err
					}
				}

				break
			}
		}
	}
	return nil
}

func (s *InterestingCadenceSpanExporter) Shutdown(_ context.Context) error {
	return nil
}

func (s *InterestingCadenceSpanExporter) WriteSpans(writer io.Writer) error {
	for _, span := range s.Spans {
		err := writeSpan(writer, span)
		if err != nil {
			return err
		}
	}
	return nil
}

func writeSpan(writer io.Writer, span otelTrace.ReadOnlySpan) error {
	_, err := fmt.Fprintf(writer, "- %s: ", span.Name())
	if err != nil {
		return err
	}
	for i, attr := range span.Attributes() {
		if i > 0 {
			_, err = fmt.Fprintf(writer, ", ")
			if err != nil {
				return err
			}
		}
		_, err = fmt.Fprintf(writer, "%s=%v", attr.Key, attr.Value.AsInterface())
		if err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(writer)
	if err != nil {
		return err
	}
	return nil
}
