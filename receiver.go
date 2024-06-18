package opentelemetrygithubactionsjunitreceiver

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v62/github"
	"github.com/joshdk/go-junit"
	"github.com/julienschmidt/httprouter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

func newTracesReceiver(cfg *Config, params receiver.CreateSettings, nextConsumer consumer.Traces) (receiver.Traces, error) {
	ghClient := github.NewClient(nil).WithAuthToken(string(cfg.Token))
	rateLimit, _, err := ghClient.RateLimit.Get(context.Background())
	if err != nil {
		return nil, err
	}
	params.Logger.Info("GitHub API rate limit", zap.Int("limit", rateLimit.GetCore().Limit), zap.Int("remaining", rateLimit.GetCore().Remaining), zap.Time("reset", rateLimit.GetCore().Reset.Time))
	return &githubactionsjunitReceiver{
		config:   cfg,
		settings: params,
		logger:   params.Logger,
		ghClient: ghClient,
	}, nil
}

type githubactionsjunitReceiver struct {
	config   *Config
	server   *http.Server
	settings receiver.CreateSettings
	logger   *zap.Logger
	ghClient *github.Client
}

func (rec *githubactionsjunitReceiver) Start(ctx context.Context, host component.Host) error {
	endpoint := fmt.Sprintf("%s%s", rec.config.ServerConfig.Endpoint, rec.config.Path)
	rec.logger.Info("Starting receiver", zap.String("endpoint", endpoint))
	listener, err := rec.config.ServerConfig.ToListener(ctx)
	if err != nil {
		return err
	}
	router := httprouter.New()
	router.POST(rec.config.Path, rec.handleEvent)
	rec.server, err = rec.config.ServerConfig.ToServer(ctx, host, rec.settings.TelemetrySettings, router)
	if err != nil {
		return err
	}
	go func() {
		if err := rec.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			rec.settings.TelemetrySettings.ReportStatus(component.NewFatalErrorEvent(err))
		}
	}()
	return nil
}

func (rec *githubactionsjunitReceiver) Shutdown(context.Context) error {
	return nil
}

func (rec *githubactionsjunitReceiver) handleEvent(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var payload []byte
	var err error
	payload, err = github.ValidatePayload(r, []byte(string(rec.config.WebhookSecret)))
	if err != nil {
		rec.logger.Error("Invalid payload", zap.Error(err))
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		rec.logger.Error("Error parsing webhook", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	switch event := event.(type) {
	case *github.WorkflowRunEvent:
		rec.handleWorkflowRunEvent(event, w, r, nil)
	default:
		{
			rec.logger.Debug("Skipping the request because it is not a workflow_job event", zap.Any("event", event))
			w.WriteHeader(http.StatusOK)
		}
	}
}

func (rec *githubactionsjunitReceiver) handleWorkflowRunEvent(workflowRunEvent *github.WorkflowRunEvent, w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	rec.logger.Debug("Handling workflow run event", zap.Int64("workflow_run.id", workflowRunEvent.WorkflowRun.GetWorkflowID()))
	if workflowRunEvent.GetAction() != "completed" {
		rec.logger.Debug("Skipping the request because it is not a completed workflow_job event", zap.Any("event", workflowRunEvent))
		w.WriteHeader(http.StatusOK)
		return
	}

	artifacts, err := getArtifacts(context.Background(), workflowRunEvent, rec.ghClient)
	if err != nil {
		rec.logger.Error("Failed to get workflow artifacts", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	var junitArtifacts []*github.Artifact
	for _, artifact := range artifacts {
		if strings.HasSuffix(artifact.GetName(), "junit") {
			junitArtifacts = append(junitArtifacts, artifact)
		}
	}
	if len(junitArtifacts) == 0 {
		rec.logger.Debug("No junit artifacts found")
		w.WriteHeader(http.StatusOK)
		return
	}
	for _, artifact := range junitArtifacts {
		err := processArtifact(rec.logger, rec.ghClient, workflowRunEvent, rec.config, artifact)
		if err != nil {
			// TODO: report error but keep processing other artifacts
			rec.logger.Error("Failed to process artifact", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

	}
}

func processArtifact(logger *zap.Logger, ghClient *github.Client, workflowRunEvent *github.WorkflowRunEvent, config *Config, artifact *github.Artifact) error {
	zipFile, err := downloadArtifact(context.Background(), ghClient, workflowRunEvent, artifact)
	if err != nil {
		return err
	}
	defer zipFile.Close()
	for _, file := range zipFile.Reader.File {
		logger.Debug("Processing file", zap.String("artifact", artifact.GetName()), zap.String("file", file.Name))
		suites := processJunitFile(file, logger)
		processSuites(suites, workflowRunEvent, config, logger)
	}
	return nil
}

func processJunitFile(file *zip.File, logger *zap.Logger) []junit.Suite {
	fileName := file.Name
	src, err := file.Open()
	if err != nil {
		logger.Error("Failed to open file in zip", zap.Error(err))
	}
	defer src.Close()

	dst, err := os.Create(filepath.Join("/tmp", fileName))
	if err != nil {
		logger.Error("Failed to create destination file", zap.Error(err))
	}
	defer dst.Close()

	// TODO: optimise if the file is too big.
	contents, err := io.ReadAll(src)
	if err != nil {
		logger.Error("Failed to read file content", zap.Error(err))
	}
	suites, err := junit.Ingest(contents)
	if err != nil {
		logger.Error("Failed to ingest JUnit file", zap.Error(err))
	}
	return suites
}

func processSuites(suites []junit.Suite, workflowRunEvent *github.WorkflowRunEvent, config *Config, logger *zap.Logger) {
	// Convert the Suites to OpenTelemetry traces
	td, err := suitesToTraces(suites, workflowRunEvent, config, logger)
	if err != nil {
		logger.Debug("Failed to convert event to traces", zap.Error(err))
	}
	// Check if the traces data contains any ResourceSpans
	if td.ResourceSpans().Len() > 0 {
		spanCount := td.SpanCount()
		logger.Debug("Unmarshaled spans", zap.Int("#spans", spanCount))

		// TODO: Pass the traces to the nextConsumer
	} else {
		logger.Debug("No spans to unmarshal or traces not initialized")
	}
}

func getArtifacts(ctx context.Context, ghEvent *github.WorkflowRunEvent, ghClient *github.Client) ([]*github.Artifact, error) {
	listArtifactsOpts := &github.ListOptions{
		PerPage: 100,
	}
	var allArtifacts []*github.Artifact
	for {
		artifacts, response, err := ghClient.Actions.ListWorkflowRunArtifacts(ctx, ghEvent.GetRepo().GetOwner().GetLogin(), ghEvent.GetRepo().GetName(), ghEvent.GetWorkflowRun().GetID(), listArtifactsOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to get workflow artifacts: %w", err)
		}
		allArtifacts = append(allArtifacts, artifacts.Artifacts...)
		if response.NextPage == 0 {
			break
		}
		listArtifactsOpts.Page = response.NextPage
	}
	return allArtifacts, nil
}

func downloadArtifact(ctx context.Context, ghClient *github.Client, event *github.WorkflowRunEvent, artifact *github.Artifact) (*zip.ReadCloser, error) {
	workflowRun := event.GetWorkflowRun()
	url, _, err := ghClient.Actions.DownloadArtifact(ctx, event.GetRepo().GetOwner().GetLogin(), event.GetRepo().GetName(), artifact.GetID(), 3)
	if err != nil {
		return nil, fmt.Errorf("failed to download artifact: %w", err)
	}
	filename := fmt.Sprintf("%s-%d-%d.zip", artifact.GetName(), workflowRun.ID, workflowRun.GetRunStartedAt().Unix())
	fp := filepath.Join(os.TempDir(), "run-artifacts", filename)
	response, err := fetchArtifact(http.DefaultClient, url.String())
	if err != nil {
		return nil, err
	}
	err = os.MkdirAll(filepath.Dir(fp), 0755)
	if err != nil {
		return nil, err
	}
	f, err := os.Create(fp)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	_, err = io.Copy(f, response)
	return zip.OpenReader(fp)
}

func fetchArtifact(httpClient *http.Client, logURL string) (io.ReadCloser, error) {
	req, err := http.NewRequest("GET", logURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get artifact: %s", resp.Status)
	}
	return resp.Body, nil
}
