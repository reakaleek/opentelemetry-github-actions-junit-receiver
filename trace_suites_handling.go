package opentelemetrygithubactionsjunitreceiver

import (
	"fmt"
	"time"

	"github.com/google/go-github/v62/github"
	"github.com/joshdk/go-junit"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// TODO: probably we can use the []suites instead
func suitesToTraces(suite junit.Suite, event *github.WorkflowRunEvent, config *Config, logger *zap.Logger) (ptrace.Traces, error) {
	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans().AppendEmpty()
	runResource := resourceSpans.Resource()
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()

	traceID, err := generateTraceID(event.GetWorkflowRun().GetID(), event.GetWorkflowRun().GetRunAttempt())
	if err != nil {
		logger.Error("Failed to generate trace ID", zap.Error(err))
		return ptrace.Traces{}, fmt.Errorf("failed to generate trace ID: %w", err)
	}

	createResourceAttributes(runResource, event, config, logger)
	// TODO: use the status from the suite object
	createRootSpan(resourceSpans, event, traceID, logger)

	parentSpanID := createParentSpan(scopeSpans, suite, event.GetWorkflowRun(), traceID, logger)
	processTests(scopeSpans, suite, event.GetWorkflowRun(), traceID, parentSpanID, logger)

	return traces, nil
}

func createRootSpan(resourceSpans ptrace.ResourceSpans, event *github.WorkflowRunEvent, traceID pcommon.TraceID, logger *zap.Logger) (pcommon.SpanID, error) {
	logger.Debug("Creating root parent span", zap.String("name", event.GetWorkflowRun().GetName()))
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	span := scopeSpans.Spans().AppendEmpty()

	rootSpanID, err := generateParentSpanID(event.GetWorkflowRun().GetID(), event.GetWorkflowRun().GetRunAttempt())
	if err != nil {
		logger.Error("Failed to generate root span ID", zap.Error(err))
		return pcommon.SpanID{}, fmt.Errorf("failed to generate root span ID: %w", err)
	}

	span.SetTraceID(traceID)
	span.SetSpanID(rootSpanID)
	span.SetName(event.GetWorkflowRun().GetName())
	span.SetKind(ptrace.SpanKindServer)
	setSpanTimes(span, event.GetWorkflowRun().GetRunStartedAt().Time, event.GetWorkflowRun().GetUpdatedAt().Time)

	switch event.WorkflowRun.GetConclusion() {
	case "success":
		span.Status().SetCode(ptrace.StatusCodeOk)
	case "failure":
		span.Status().SetCode(ptrace.StatusCodeError)
	default:
		span.Status().SetCode(ptrace.StatusCodeUnset)
	}

	span.Status().SetMessage(event.GetWorkflowRun().GetConclusion())

	// Attempt to link to previous trace ID if applicable
	if event.GetWorkflowRun().GetPreviousAttemptURL() != "" && event.GetWorkflowRun().GetRunAttempt() > 1 {
		logger.Debug("Linking to previous trace ID for WorkflowRunEvent")
		previousRunAttempt := event.GetWorkflowRun().GetRunAttempt() - 1
		previousTraceID, err := generateTraceID(event.GetWorkflowRun().GetID(), previousRunAttempt)
		if err != nil {
			logger.Error("Failed to generate previous trace ID", zap.Error(err))
		} else {
			link := span.Links().AppendEmpty()
			link.SetTraceID(previousTraceID)
			logger.Debug("Successfully linked to previous trace ID", zap.String("previousTraceID", previousTraceID.String()))
		}
	}

	return rootSpanID, nil
}

func createParentSpan(scopeSpans ptrace.ScopeSpans, suite junit.Suite, run *github.WorkflowRun, traceID pcommon.TraceID, logger *zap.Logger) pcommon.SpanID {
	logger.Debug("Creating parent span", zap.String("name", run.GetName()))
	span := scopeSpans.Spans().AppendEmpty()
	span.SetTraceID(traceID)

	parentSpanID, _ := generateParentSpanID(*run.ID, int(*run.RunAttempt))
	span.SetParentSpanID(parentSpanID)

	jobSpanID, _ := generateJobSpanID(*run.ID, int(*run.RunAttempt), *run.Name)
	span.SetSpanID(jobSpanID)

	// TODO: package + name?
	span.SetName(suite.Name)
	span.SetKind(ptrace.SpanKindInternal)

	span.Attributes().PutStr(TestsSuiteName, suite.Name)
	span.Attributes().PutStr(TestsSystemErr, suite.SystemErr)
	span.Attributes().PutStr(TestsSystemOut, suite.SystemOut)
	// TODO: support int64
	span.Attributes().PutInt(TestsDuration, suite.Totals.Duration.Milliseconds())

	// TODO: set span time
	setSpanTimes(span, run.RunStartedAt.Time, run.GetUpdatedAt().Time)

	anyFailure := false
	for _, test := range suite.Tests {
		if test.Error != nil {
			anyFailure = true
		}
	}

	// QUESTION: what status if any skipped tests?
	if anyFailure {
		span.Status().SetCode(ptrace.StatusCodeError)
	} else {
		span.Status().SetCode(ptrace.StatusCodeOk)
	}

	return span.SpanID()
}

func createTestSpan(scopeSpans ptrace.ScopeSpans, test junit.Test, run *github.WorkflowRun, traceID pcommon.TraceID, parentSpanID pcommon.SpanID, logger *zap.Logger, stepNumber ...int) pcommon.SpanID {
	logger.Debug("Processing span", zap.String("test_name", test.Name))
	span := scopeSpans.Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetParentSpanID(parentSpanID)

	var spanID pcommon.SpanID

	// Set attributes
	span.Attributes().PutInt(TestDuration, test.Duration.Milliseconds())
	span.Attributes().PutStr(TestClassName, test.Classname)
	span.Attributes().PutStr(TestMessage, test.Message)
	span.Attributes().PutStr(TestStatus, string(test.Status))
	span.Attributes().PutStr(TestSystemErr, test.SystemErr)
	span.Attributes().PutStr(TestSystemOut, test.SystemOut)
	if test.Error != nil {
		span.Attributes().PutStr(TestError, test.Error.Error())
	}
	spanID, _ = generateTestSpanID(run.GetID(), int(run.GetRunAttempt()), run.GetName(), test.Classname, test.Name)

	span.SetSpanID(spanID)

	// TODO: set span time
	//setSpanTimes(span, step.GetStartedAt().Time, step.GetCompletedAt().Time)

	span.SetName(test.Name)
	span.SetKind(ptrace.SpanKindInternal)

	switch test.Status {
	case "passed":
		span.Status().SetCode(ptrace.StatusCodeOk)
	case "skipped":
		span.Status().SetCode(ptrace.StatusCodeOk)
	default:
		span.Status().SetCode(ptrace.StatusCodeError)
	}

	return span.SpanID()
}

func processTests(scopeSpans ptrace.ScopeSpans, suite junit.Suite, run *github.WorkflowRun, traceID pcommon.TraceID, parentSpanID pcommon.SpanID, logger *zap.Logger) {
	for _, test := range suite.Tests {
		createTestSpan(scopeSpans, test, run, traceID, parentSpanID, logger)
	}
}

func setSpanTimes(span ptrace.Span, start, end time.Time) {
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(start))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(end))
}
