package trace

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

func BenchmarkStartSpanFromParent(b *testing.B) {
	tracer, err := NewTracer(zerolog.Logger{}, "test", string(flow.Localnet), 0)
	require.NoError(b, err)

	tracer.Ready()
	defer tracer.Done()

	span, _ := tracer.StartBlockSpan(context.Background(), flow.Identifier{}, "test")
	defer span.End()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := tracer.StartSpanFromParent(span, SpanName("test"))
		s.End()
	}
	b.StopTimer()
}

func BenchmarkStartBlockSpan(b *testing.B) {
	tracer, err := NewTracer(zerolog.Logger{}, "test", string(flow.Localnet), 0)
	require.NoError(b, err)

	tracer.Ready()
	defer tracer.Done()

	for _, t := range []struct {
		name string
		n    int
	}{
		{name: "cacheHit", n: 100},
		{name: "cacheMiss", n: 100000},
	} {
		t := t
		b.Run(t.name, func(b *testing.B) {
			randomIDs := make([]flow.Identifier, 0, t.n)
			for i := 0; i < t.n; i++ {
				var randomBytes [flow.IdentifierLen]byte
				_, err := rand.Read(randomBytes[:])
				require.NoError(b, err)
				randomIDs = append(randomIDs, flow.Identifier(randomBytes))
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				span, _ := tracer.StartBlockSpan(context.Background(), randomIDs[i%t.n], "test")
				span.End()
			}
			b.StopTimer()
		})
	}
}
