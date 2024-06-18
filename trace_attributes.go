package opentelemetrygithubactionsjunitreceiver

import (
	"strings"
	"time"

	"github.com/google/go-github/v62/github"
	"github.com/joshdk/go-junit"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"go.uber.org/zap"
)

func createResourceAttributesTestSuite(resource pcommon.Resource, suite junit.Suite, config *Config) {
	attrs := resource.Attributes()
	attrs.PutStr(string(semconv.CodeNamespaceKey), suite.Package)
	attrs.PutStr("tests.suite.suitename", suite.Name)
	attrs.PutStr("tests.suite.suitename", suite.Name)
	attrs.PutStr("tests.suite.systemerr", suite.SystemErr)
	attrs.PutStr("tests.suite.systemout", suite.SystemOut)
	attrs.PutInt("tests.suite.duration", suite.Totals.Duration.Milliseconds())
}

func createResourceAttributesTest(resource pcommon.Resource, test junit.Test, config *Config) {
	attrs := resource.Attributes()
	attrs.PutInt("tests.case.duration", test.Duration.Milliseconds())
	attrs.PutStr(string(semconv.CodeFunctionKey), test.Name)
	attrs.PutStr("tests.case.classname", test.Classname)
	attrs.PutStr("tests.case.message", test.Message)
	attrs.PutStr("tests.case.status", string(test.Status))
	attrs.PutStr("tests.case.systemerr", test.SystemErr)
	attrs.PutStr("tests.case.systemout", test.SystemOut)
	if test.Error != nil {
		attrs.PutStr("tests.case.error", test.Error.Error())
	}
}

func createResourceAttributes(resource pcommon.Resource, e *github.WorkflowRunEvent, config *Config, logger *zap.Logger) {
	attrs := resource.Attributes()

	serviceName := generateServiceName(config, e.GetRepo().GetFullName())
	attrs.PutStr("service.name", serviceName)

	attrs.PutStr("ci.github.workflow.run.actor.login", e.GetWorkflowRun().GetActor().GetLogin())

	attrs.PutStr("ci.github.workflow.run.conclusion", e.GetWorkflowRun().GetConclusion())
	attrs.PutStr("ci.github.workflow.run.created_at", e.GetWorkflowRun().GetCreatedAt().Format(time.RFC3339))
	attrs.PutStr("ci.github.workflow.run.display_title", e.GetWorkflowRun().GetDisplayTitle())
	attrs.PutStr("ci.github.workflow.run.event", e.GetWorkflowRun().GetEvent())
	attrs.PutStr("ci.github.workflow.run.head_branch", e.GetWorkflowRun().GetHeadBranch())
	attrs.PutStr("ci.github.workflow.run.head_sha", e.GetWorkflowRun().GetHeadSHA())
	attrs.PutStr("ci.github.workflow.run.html_url", e.GetWorkflowRun().GetHTMLURL())
	attrs.PutInt("ci.github.workflow.run.id", e.GetWorkflowRun().GetID())
	attrs.PutStr("ci.github.workflow.run.name", e.GetWorkflowRun().GetName())
	attrs.PutStr("ci.github.workflow.run.path", e.GetWorkflow().GetPath())

	if e.GetWorkflowRun().GetPreviousAttemptURL() != "" {
		htmlURL := transformGitHubAPIURL(e.GetWorkflowRun().GetPreviousAttemptURL())
		attrs.PutStr("ci.github.workflow.run.previous_attempt_url", htmlURL)
	}

	if len(e.GetWorkflowRun().ReferencedWorkflows) > 0 {
		var referencedWorkflows []string
		for _, workflow := range e.GetWorkflowRun().ReferencedWorkflows {
			referencedWorkflows = append(referencedWorkflows, workflow.GetPath())
		}
		attrs.PutStr("ci.github.workflow.run.referenced_workflows", strings.Join(referencedWorkflows, ";"))
	}

	attrs.PutInt("ci.github.workflow.run.run_attempt", int64(e.GetWorkflowRun().GetRunAttempt()))
	attrs.PutStr("ci.github.workflow.run.run_started_at", e.GetWorkflowRun().RunStartedAt.Format(time.RFC3339))
	attrs.PutStr("ci.github.workflow.run.status", e.GetWorkflowRun().GetStatus())
	attrs.PutStr("ci.github.workflow.run.sender.login", e.GetSender().GetLogin())
	attrs.PutStr("ci.github.workflow.run.triggering_actor.login", e.GetWorkflowRun().GetTriggeringActor().GetLogin())
	attrs.PutStr("ci.github.workflow.run.updated_at", e.GetWorkflowRun().GetUpdatedAt().Format(time.RFC3339))
	duration := e.GetWorkflowRun().GetUpdatedAt().Sub(e.GetWorkflowRun().GetRunStartedAt().Time)
	attrs.PutInt("ci.github.workflow.run.duration_millis", duration.Milliseconds())

	attrs.PutStr("ci.system", "github")

	attrs.PutStr("scm.system", "git")

	attrs.PutStr("scm.git.head_branch", e.GetWorkflowRun().GetHeadBranch())
	attrs.PutStr("scm.git.head_commit.author.email", e.GetWorkflowRun().GetHeadCommit().GetAuthor().GetEmail())
	attrs.PutStr("scm.git.head_commit.author.name", e.GetWorkflowRun().GetHeadCommit().GetAuthor().GetName())
	attrs.PutStr("scm.git.head_commit.committer.email", e.GetWorkflowRun().GetHeadCommit().GetCommitter().GetEmail())
	attrs.PutStr("scm.git.head_commit.committer.name", e.GetWorkflowRun().GetHeadCommit().GetCommitter().GetName())
	attrs.PutStr("scm.git.head_commit.message", e.GetWorkflowRun().GetHeadCommit().GetMessage())
	attrs.PutStr("scm.git.head_commit.timestamp", e.GetWorkflowRun().GetHeadCommit().GetTimestamp().Format(time.RFC3339))
	attrs.PutStr("scm.git.head_sha", e.GetWorkflowRun().GetHeadSHA())

	if len(e.GetWorkflowRun().PullRequests) > 0 {
		var prUrls []string
		for _, pr := range e.GetWorkflowRun().PullRequests {
			prUrls = append(prUrls, convertPRURL(pr.GetURL()))
		}
		attrs.PutStr("scm.git.pull_requests.url", strings.Join(prUrls, ";"))
	}

	attrs.PutStr("scm.git.repo", e.GetRepo().GetFullName())

}
