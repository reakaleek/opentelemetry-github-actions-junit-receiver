package opentelemetrygithubactionsjunitreceiver

import (
	crand "crypto/rand"
	"encoding/binary"
	"github.com/joshdk/go-junit"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"math/rand"
	"time"
)

func convertSuiteToTraces(suites []junit.Suite, startTimestamp time.Time, scopeSpans ptrace.ScopeSpans, rootSpan ptrace.Span) {
	for _, suite := range suites {
		suiteSpan := scopeSpans.Spans().AppendEmpty()
		suiteSpan.SetName(suite.Name)
		suiteSpan.SetKind(ptrace.SpanKindInternal)
		suiteSpan.SetStartTimestamp(pcommon.NewTimestampFromTime(startTimestamp))
		suiteSpan.SetEndTimestamp(pcommon.NewTimestampFromTime(startTimestamp.Add(suite.Totals.Duration)))
		suiteSpan.SetTraceID(rootSpan.TraceID())
		suiteSpan.SetSpanID(NewSpanID())
		suiteSpan.SetParentSpanID(rootSpan.SpanID())
		createResourceAttributesTestSuite(suiteSpan, suite)
		for _, test := range suite.Tests {
			span := scopeSpans.Spans().AppendEmpty()
			span.SetName(test.Name)
			span.SetKind(ptrace.SpanKindInternal)
			span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTimestamp))
			span.SetEndTimestamp(pcommon.NewTimestampFromTime(startTimestamp.Add(test.Duration)))
			span.SetTraceID(suiteSpan.TraceID())
			span.SetSpanID(NewSpanID())
			span.SetParentSpanID(suiteSpan.SpanID())
			createResourceAttributesTest(span, test)
			switch test.Status {
			case junit.StatusPassed:
				span.Status().SetCode(ptrace.StatusCodeOk)
			case junit.StatusFailed:
				span.Status().SetCode(ptrace.StatusCodeError)
			case junit.StatusSkipped:
				span.Status().SetCode(ptrace.StatusCodeUnset)
			}
		}
	}
}

func NewSpanID() pcommon.SpanID {
	var rngSeed int64
	_ = binary.Read(crand.Reader, binary.LittleEndian, &rngSeed)
	randSource := rand.New(rand.NewSource(rngSeed))
	var sid [8]byte
	randSource.Read(sid[:])
	spanID := pcommon.SpanID(sid)
	return spanID
}
