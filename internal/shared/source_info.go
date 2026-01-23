package shared

import (
	"fmt"
	"os"
	"time"
)

// CollectPushInfo collects metadata about the push operation from the environment
func CollectPushInfo() PushInfo {
	return PushInfo{
		PushedAt: time.Now().UTC().Format(time.RFC3339),
		Source:   collectPushSource(),
	}
}

// collectPushSource detects the execution environment and collects relevant info
func collectPushSource() PushSource {
	// Check if running in GitHub Actions
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		return collectGitHubActionsSource()
	}

	// Default to local execution
	return PushSource{
		Type: "local",
	}
}

// collectGitHubActionsSource collects GitHub Actions specific information
func collectGitHubActionsSource() PushSource {
	serverURL := os.Getenv("GITHUB_SERVER_URL")
	repository := os.Getenv("GITHUB_REPOSITORY")
	runID := os.Getenv("GITHUB_RUN_ID")

	var runURL string
	if serverURL != "" && repository != "" && runID != "" {
		runURL = fmt.Sprintf("%s/%s/actions/runs/%s", serverURL, repository, runID)
	}

	return PushSource{
		Type:       "github_actions",
		Repository: repository,
		Workflow:   os.Getenv("GITHUB_WORKFLOW"),
		RunID:      runID,
		RunURL:     runURL,
		Actor:      os.Getenv("GITHUB_ACTOR"),
		SHA:        os.Getenv("GITHUB_SHA"),
		Ref:        os.Getenv("GITHUB_REF"),
	}
}
