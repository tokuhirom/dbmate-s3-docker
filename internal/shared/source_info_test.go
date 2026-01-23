package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectPushInfo_Local(t *testing.T) {
	// Use t.Setenv which automatically restores on cleanup
	// Setting to empty string effectively unsets for our detection logic
	t.Setenv("GITHUB_ACTIONS", "")

	info := CollectPushInfo()

	assert.NotEmpty(t, info.PushedAt)
	assert.Equal(t, "local", info.Source.Type)
	assert.Empty(t, info.Source.Repository)
	assert.Empty(t, info.Source.Workflow)
}

func TestCollectPushInfo_GitHubActions(t *testing.T) {
	// Set GitHub Actions env vars using t.Setenv
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_REPOSITORY", "tokuhirom/dbmate-deployer")
	t.Setenv("GITHUB_WORKFLOW", "Deploy Migrations")
	t.Setenv("GITHUB_RUN_ID", "12345678")
	t.Setenv("GITHUB_SERVER_URL", "https://github.com")
	t.Setenv("GITHUB_ACTOR", "tokuhirom")
	t.Setenv("GITHUB_SHA", "abc123def456")
	t.Setenv("GITHUB_REF", "refs/heads/main")

	info := CollectPushInfo()

	assert.NotEmpty(t, info.PushedAt)
	assert.Equal(t, "github_actions", info.Source.Type)
	assert.Equal(t, "tokuhirom/dbmate-deployer", info.Source.Repository)
	assert.Equal(t, "Deploy Migrations", info.Source.Workflow)
	assert.Equal(t, "12345678", info.Source.RunID)
	assert.Equal(t, "https://github.com/tokuhirom/dbmate-deployer/actions/runs/12345678", info.Source.RunURL)
	assert.Equal(t, "tokuhirom", info.Source.Actor)
	assert.Equal(t, "abc123def456", info.Source.SHA)
	assert.Equal(t, "refs/heads/main", info.Source.Ref)
}

func TestCollectPushInfo_GitHubActions_PartialEnv(t *testing.T) {
	// Set only GITHUB_ACTIONS (simulate minimal GHA environment)
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_REPOSITORY", "")
	t.Setenv("GITHUB_WORKFLOW", "")
	t.Setenv("GITHUB_RUN_ID", "")
	t.Setenv("GITHUB_SERVER_URL", "")
	t.Setenv("GITHUB_ACTOR", "")
	t.Setenv("GITHUB_SHA", "")
	t.Setenv("GITHUB_REF", "")

	info := CollectPushInfo()

	assert.Equal(t, "github_actions", info.Source.Type)
	assert.Empty(t, info.Source.RunURL) // URL should be empty when components are missing
}

func TestCollectPushInfo_Timestamp(t *testing.T) {
	info := CollectPushInfo()

	// Verify timestamp is in RFC3339 format
	require.NotEmpty(t, info.PushedAt)
	assert.Contains(t, info.PushedAt, "T") // RFC3339 contains T separator
	assert.Contains(t, info.PushedAt, "Z") // UTC timezone
}
