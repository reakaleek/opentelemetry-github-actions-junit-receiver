package opentelemetrygithubactionsjunitreceiver

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"github.com/google/go-github/v62/github"
	"github.com/julienschmidt/httprouter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
	"io"
	"net/http"
	"strings"
)

func newTracesReceiver(cfg *Config, params receiver.CreateSettings, nextConsumer consumer.Traces) (receiver.Traces, error) {
	return &githubactionsjunitReceiver{
		config:   cfg,
		settings: params,
		logger:   params.Logger,
	}, nil
}

type githubactionsjunitReceiver struct {
	config   *Config
	server   *http.Server
	settings receiver.CreateSettings
	logger   *zap.Logger
}

func (rec *githubactionsjunitReceiver) Start(ctx context.Context, host component.Host) error {
	endpoint := fmt.Sprintf("%s%s", rec.config.ServerConfig.Endpoint, rec.config.Path)
	rec.logger.Info("Starting receiver", zap.String("endpoint", endpoint))
	listener, err := rec.config.ServerConfig.ToListener(ctx)
	if err != nil {
		return err
	}
	router := httprouter.New()
	router.POST(rec.config.Path, func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		w.WriteHeader(200)
	})
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
	rec.logger.Debug("Handling workflow run event", zap.Any("event", workflowRunEvent))
	if workflowRunEvent.GetAction() != "completed" {
		rec.logger.Debug("Skipping the request because it is not a completed workflow_job event", zap.Any("event", workflowRunEvent))
		w.WriteHeader(http.StatusOK)
		return
	}
	ghClient := github.NewClient(nil).WithAuthToken(string(rec.config.Token))

	artifacts, err := getArtifacts(context.Background(), workflowRunEvent, ghClient)
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

func downloadArtifact(ctx context.Context, ghClient *github.Client, owner string, repo string, artifact *github.Artifact) (*zip.ReadCloser, error) {
	// TODO
	_, _, err := ghClient.Actions.DownloadArtifact(ctx, owner, repo, artifact.GetID(), 3)
	if err != nil {
		return nil, fmt.Errorf("failed to download artifact: %w", err)
	}

	return nil, nil
	//
	//return fetchArtifact(ghClient.Client(), url.String()), nil
	//
	//return artifact, nil
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
