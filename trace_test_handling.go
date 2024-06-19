package opentelemetrygithubactionsjunitreceiver

import (
	"fmt"

	"github.com/google/go-github/v62/github"
	"github.com/joshdk/go-junit"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func suitesToTraces(suites []junit.Suite, e *github.WorkflowRunEvent, config *Config, logger *zap.Logger) (ptrace.Traces, error) {
	logger.Debug("Determining event")
	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans().AppendEmpty()
	// NOTE: Avoid too much attributes to help with debugging the new changes. To be commented out later.
	//runResource := resourceSpans.Resource()
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	suiteResource := resourceSpans.Resource()

	logger.Info("Processing WorkflowRunEvent", zap.Int64("workflow_id", e.GetWorkflowRun().GetID()), zap.String("workflow_name", e.GetWorkflowRun().GetName()), zap.String("repo", e.GetRepo().GetFullName()))

	traceID, err := generateTraceID(e.GetWorkflowRun().GetID(), e.GetWorkflowRun().GetRunAttempt())
	if err != nil {
		logger.Error("Failed to generate trace ID", zap.Error(err))
		return ptrace.Traces{}, fmt.Errorf("failed to generate trace ID: %w", err)
	}

	// NOTE: Avoid too much attributes to help with debugging the new changes. To be commented out later.
	//createResourceAttributes(runResource, e, config)
	createRootSpan(resourceSpans, e, traceID, logger)

	for _, suite := range suites {
		createResourceAttributesTestSuite(suiteResource, suite, config)

		traceID, err := generateTraceID(*e.WorkflowRun.ID, int(*e.WorkflowRun.RunAttempt))
		if err != nil {
			// QUESTION: Should we return an error here?
			logger.Error("Failed to generate trace ID", zap.Error(err))
			return ptrace.Traces{}, fmt.Errorf("failed to generate trace ID: %w", err)
		}

		createParentSpan(scopeSpans, suite, e, traceID, logger)

		// TODO: create child spans for each test case
	}

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

	// TODO: Set status based on conclusion or base don the number of failed tests?
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

func createParentSpan(scopeSpans ptrace.ScopeSpans, suite junit.Suite, event *github.WorkflowRunEvent, traceID pcommon.TraceID, logger *zap.Logger) pcommon.SpanID {
	logger.Debug("Creating parent span", zap.String("name", string(*event.WorkflowRun.Name)))
	span := scopeSpans.Spans().AppendEmpty()
	span.SetTraceID(traceID)

	parentSpanID, _ := generateParentSpanID(*event.WorkflowRun.ID, int(*event.WorkflowRun.RunAttempt))
	span.SetParentSpanID(parentSpanID)

	jobSpanID, _ := generateJobSpanID(*event.WorkflowRun.ID, int(*event.WorkflowRun.RunAttempt), *event.GetWorkflowRun().Name)
	span.SetSpanID(jobSpanID)

	span.SetName(suite.Name)
	span.SetKind(ptrace.SpanKindInternal)

	// NOTE: JUnit does not provide when the test started but we can assume
	//       it started when the workflow run started and ended by adding the duration.
	setSpanTimes(span, event.GetWorkflowRun().GetRunStartedAt().Time, event.GetWorkflowRun().GetRunStartedAt().Time.Add(suite.Totals.Duration))

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
