//go:build integration

package once

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tokuhirom/dbmate-s3-docker/internal/shared/testhelpers"
)

func init() {
	// Set AWS credentials for LocalStack (used by Execute which creates its own S3 client)
	os.Setenv("AWS_ACCESS_KEY_ID", "test")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	os.Setenv("AWS_DEFAULT_REGION", "us-east-1")
}

func TestOnce_Execute_SuccessfulMigration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	env := testhelpers.SetupTestEnvironment(ctx, t)

	// Upload test migrations to S3
	migrationsDir := filepath.Join("..", "testdata", "migrations", "valid")
	env.UploadMigrationsFromDir(ctx, migrationsDir)

	// Execute once command
	cmd := &Cmd{
		DatabaseURL:  env.DatabaseURL,
		S3Bucket:     env.S3Bucket,
		S3PathPrefix: "migrations/",
	}

	err := Execute(cmd, env.S3EndpointURL, "")
	require.NoError(t, err)

	// Verify result was uploaded to S3
	assert.True(t, env.ResultExists(ctx, "20240101000000"))

	// Get and verify result
	result := env.GetResult(ctx, "20240101000000")
	assert.Equal(t, "success", result["status"])
	assert.Equal(t, "20240101000000", result["version"])

	// Verify table was created
	env.AssertTableExists(t, "test_table")
}

func TestOnce_Execute_NoUnappliedVersions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	env := testhelpers.SetupTestEnvironment(ctx, t)

	// Execute once command on empty bucket
	cmd := &Cmd{
		DatabaseURL:  env.DatabaseURL,
		S3Bucket:     env.S3Bucket,
		S3PathPrefix: "migrations/",
	}

	err := Execute(cmd, env.S3EndpointURL, "")

	// Should return nil when no unapplied versions found
	assert.NoError(t, err)
}

func TestOnce_Execute_AlreadyAppliedVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	env := testhelpers.SetupTestEnvironment(ctx, t)

	// Upload test migrations
	migrationsDir := filepath.Join("..", "testdata", "migrations", "valid")
	env.UploadMigrationsFromDir(ctx, migrationsDir)

	// Upload result to mark version as applied
	env.UploadResult(ctx, "20240101000000", testhelpers.SuccessResult("20240101000000", "Already applied"))

	// Execute once command
	cmd := &Cmd{
		DatabaseURL:  env.DatabaseURL,
		S3Bucket:     env.S3Bucket,
		S3PathPrefix: "migrations/",
	}

	err := Execute(cmd, env.S3EndpointURL, "")

	// Should succeed with message that all versions are applied
	assert.NoError(t, err)
}
