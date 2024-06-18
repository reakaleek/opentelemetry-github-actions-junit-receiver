package opentelemetrygithubactionsjunitreceiver

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/go-github/v62/github"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func checkDuplicateStepNames(steps []*github.TaskStep) map[string]int {
	nameCount := make(map[string]int)
	for _, step := range steps {
		nameCount[step.GetName()]++
	}
	return nameCount
}

func convertPRURL(apiURL string) string {
	apiURL = strings.Replace(apiURL, "/repos", "", 1)
	apiURL = strings.Replace(apiURL, "/pulls", "/pull", 1)
	return strings.Replace(apiURL, "api.", "", 1)
}
func generateTraceID(runID int64, runAttempt int) (pcommon.TraceID, error) {
	input := fmt.Sprintf("%d%dt", runID, runAttempt)
	hash := sha256.Sum256([]byte(input))
	traceIDHex := hex.EncodeToString(hash[:])

	var traceID pcommon.TraceID
	_, err := hex.Decode(traceID[:], []byte(traceIDHex[:32]))
	if err != nil {
		return pcommon.TraceID{}, err
	}

	return traceID, nil
}

func generateJobSpanID(runID int64, runAttempt int, job string) (pcommon.SpanID, error) {
	input := fmt.Sprintf("%d%d%s", runID, runAttempt, job)
	hash := sha256.Sum256([]byte(input))
	spanIDHex := hex.EncodeToString(hash[:])

	var spanID pcommon.SpanID
	_, err := hex.Decode(spanID[:], []byte(spanIDHex[16:32]))
	if err != nil {
		return pcommon.SpanID{}, err
	}

	return spanID, nil
}

func generateParentSpanID(runID int64, runAttempt int) (pcommon.SpanID, error) {
	input := fmt.Sprintf("%d%ds", runID, runAttempt)
	hash := sha256.Sum256([]byte(input))
	spanIDHex := hex.EncodeToString(hash[:])

	var spanID pcommon.SpanID
	_, err := hex.Decode(spanID[:], []byte(spanIDHex[16:32]))
	if err != nil {
		return pcommon.SpanID{}, err
	}

	return spanID, nil
}

func generateServiceName(config *Config, fullName string) string {
	formattedName := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(fullName, "/", "-"), "_", "-"))
	return fmt.Sprintf("%s", formattedName)
}

func generateStepSpanID(runID int64, runAttempt int, jobName, stepName string, stepNumber ...int) (pcommon.SpanID, error) {
	var input string
	if len(stepNumber) > 0 && stepNumber[0] > 0 {
		input = fmt.Sprintf("%d%d%s%s%d", runID, runAttempt, jobName, stepName, stepNumber[0])
	} else {
		input = fmt.Sprintf("%d%d%s%s", runID, runAttempt, jobName, stepName)
	}
	hash := sha256.Sum256([]byte(input))
	spanIDHex := hex.EncodeToString(hash[:])

	var spanID pcommon.SpanID
	_, err := hex.Decode(spanID[:], []byte(spanIDHex[16:32]))
	if err != nil {
		return pcommon.SpanID{}, err
	}

	return spanID, nil
}

func transformGitHubAPIURL(apiURL string) string {
	htmlURL := strings.Replace(apiURL, "api.github.com/repos", "github.com", 1)
	return htmlURL
}
