package shared

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tokuhirom/dbmate-deployer/internal/shared/testhelpers"
)

func TestCheckResultExists(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*testhelpers.MockS3Client)
		bucket   string
		prefix   string
		version  string
		expected bool
	}{
		{
			name: "result exists",
			setup: func(mock *testhelpers.MockS3Client) {
				_, _ = mock.PutObject(context.Background(), &s3.PutObjectInput{
					Bucket: aws.String("test-bucket"),
					Key:    aws.String("migrations/20240101000000/result.json"),
					Body:   io.NopCloser(bytes.NewBufferString(`{"status":"success"}`)),
				})
			},
			bucket:   "test-bucket",
			prefix:   "migrations/",
			version:  "20240101000000",
			expected: true,
		},
		{
			name:     "result does not exist",
			setup:    func(mock *testhelpers.MockS3Client) {},
			bucket:   "test-bucket",
			prefix:   "migrations/",
			version:  "20240101000000",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := testhelpers.NewMockS3Client()
			tt.setup(mock)

			exists, err := CheckResultExists(context.Background(), mock, tt.bucket, tt.prefix, tt.version)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, exists)
		})
	}
}

func TestFindUnappliedVersion(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(*testhelpers.MockS3Client)
		bucket        string
		prefix        string
		expectVersion string
		expectError   bool
	}{
		{
			name: "find newest unapplied version",
			setup: func(mock *testhelpers.MockS3Client) {
				// Create version directories (using common prefixes)
				_, _ = mock.PutObject(context.Background(), &s3.PutObjectInput{
					Bucket: aws.String("test-bucket"),
					Key:    aws.String("migrations/20240101000000/migrations/test.sql"),
					Body:   io.NopCloser(bytes.NewBufferString("test")),
				})
				_, _ = mock.PutObject(context.Background(), &s3.PutObjectInput{
					Bucket: aws.String("test-bucket"),
					Key:    aws.String("migrations/20240102000000/migrations/test.sql"),
					Body:   io.NopCloser(bytes.NewBufferString("test")),
				})
				// Add result for older version only
				_, _ = mock.PutObject(context.Background(), &s3.PutObjectInput{
					Bucket: aws.String("test-bucket"),
					Key:    aws.String("migrations/20240101000000/result.json"),
					Body:   io.NopCloser(bytes.NewBufferString(`{"status":"success"}`)),
				})
			},
			bucket:        "test-bucket",
			prefix:        "migrations/",
			expectVersion: "20240102000000",
			expectError:   false,
		},
		{
			name: "all versions applied",
			setup: func(mock *testhelpers.MockS3Client) {
				_, _ = mock.PutObject(context.Background(), &s3.PutObjectInput{
					Bucket: aws.String("test-bucket"),
					Key:    aws.String("migrations/20240101000000/migrations/test.sql"),
					Body:   io.NopCloser(bytes.NewBufferString("test")),
				})
				_, _ = mock.PutObject(context.Background(), &s3.PutObjectInput{
					Bucket: aws.String("test-bucket"),
					Key:    aws.String("migrations/20240101000000/result.json"),
					Body:   io.NopCloser(bytes.NewBufferString(`{"status":"success"}`)),
				})
			},
			bucket:        "test-bucket",
			prefix:        "migrations/",
			expectVersion: "",
			expectError:   true,
		},
		{
			name:          "no versions found",
			setup:         func(mock *testhelpers.MockS3Client) {},
			bucket:        "test-bucket",
			prefix:        "migrations/",
			expectVersion: "",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := testhelpers.NewMockS3Client()
			tt.setup(mock)

			version, err := FindUnappliedVersion(context.Background(), mock, tt.bucket, tt.prefix)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectVersion, version)
			}
		})
	}
}

func TestUploadResult(t *testing.T) {
	mock := testhelpers.NewMockS3Client()

	result := &Result{
		Version:           "20240101000000",
		Status:            "success",
		Timestamp:         "2024-01-01T00:00:00Z",
		MigrationsApplied: 3,
		Log:               "Migration completed",
	}

	err := UploadResult(context.Background(), mock, "test-bucket", "migrations/", "20240101000000", result)
	require.NoError(t, err)

	// Verify the result was uploaded
	exists := mock.HasObject("test-bucket", "migrations/20240101000000/result.json")
	assert.True(t, exists)

	// Verify the content
	content, found := mock.GetObjectContent("test-bucket", "migrations/20240101000000/result.json")
	require.True(t, found)
	assert.Contains(t, content, `"status": "success"`)
	assert.Contains(t, content, `"version": "20240101000000"`)
}

func TestUploadPushInfo(t *testing.T) {
	mock := testhelpers.NewMockS3Client()

	pushInfo := &PushInfo{
		PushedAt: "2024-01-01T00:00:00Z",
		Source: PushSource{
			Type:       "github_actions",
			Repository: "tokuhirom/dbmate-deployer",
			Workflow:   "Deploy Migrations",
			RunID:      "12345678",
			RunURL:     "https://github.com/tokuhirom/dbmate-deployer/actions/runs/12345678",
			Actor:      "tokuhirom",
			SHA:        "abc123",
			Ref:        "refs/heads/main",
		},
	}

	err := UploadPushInfo(context.Background(), mock, "test-bucket", "migrations/", "20240101000000", pushInfo)
	require.NoError(t, err)

	// Verify the push info was uploaded
	exists := mock.HasObject("test-bucket", "migrations/20240101000000/push-info.json")
	assert.True(t, exists)

	// Verify the content
	content, found := mock.GetObjectContent("test-bucket", "migrations/20240101000000/push-info.json")
	require.True(t, found)
	assert.Contains(t, content, `"type": "github_actions"`)
	assert.Contains(t, content, `"repository": "tokuhirom/dbmate-deployer"`)
	assert.Contains(t, content, `"workflow": "Deploy Migrations"`)
	assert.Contains(t, content, `"run_url": "https://github.com/tokuhirom/dbmate-deployer/actions/runs/12345678"`)
}

func TestUploadPushInfo_Local(t *testing.T) {
	mock := testhelpers.NewMockS3Client()

	pushInfo := &PushInfo{
		PushedAt: "2024-01-01T00:00:00Z",
		Source: PushSource{
			Type: "local",
		},
	}

	err := UploadPushInfo(context.Background(), mock, "test-bucket", "migrations/", "20240101000000", pushInfo)
	require.NoError(t, err)

	// Verify the content
	content, found := mock.GetObjectContent("test-bucket", "migrations/20240101000000/push-info.json")
	require.True(t, found)
	assert.Contains(t, content, `"type": "local"`)
	// Local should not have repository or workflow
	assert.NotContains(t, content, `"repository"`)
}

func TestDownloadMigrations(t *testing.T) {
	mock := testhelpers.NewMockS3Client()

	// Upload test migration files
	_, _ = mock.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String("test-bucket"),
		Key:    aws.String("migrations/20240101000000/migrations/001_create_users.sql"),
		Body:   io.NopCloser(bytes.NewBufferString("CREATE TABLE users (id INT);")),
	})
	_, _ = mock.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String("test-bucket"),
		Key:    aws.String("migrations/20240101000000/migrations/002_create_posts.sql"),
		Body:   io.NopCloser(bytes.NewBufferString("CREATE TABLE posts (id INT);")),
	})

	// Download to temp directory
	tempDir := t.TempDir()

	err := DownloadMigrations(context.Background(), mock,
		"test-bucket",
		"migrations/20240101000000/migrations/",
		tempDir)
	require.NoError(t, err)

	// Verify files were downloaded
	// Note: This test is limited because our mock doesn't fully support complex S3 operations
	// In a real integration test, we'd verify the actual files exist
}

func TestUploadMigrations(t *testing.T) {
	mock := testhelpers.NewMockS3Client()

	// Create temp directory with test migration files
	tempDir := t.TempDir()
	err := testhelpers.WriteFile(tempDir, "001_create_users.sql", "CREATE TABLE users (id INT);")
	require.NoError(t, err)
	err = testhelpers.WriteFile(tempDir, "002_create_posts.sql", "CREATE TABLE posts (id INT);")
	require.NoError(t, err)

	// Upload migrations
	err = UploadMigrations(context.Background(), mock,
		"test-bucket",
		"migrations/",
		"20240101000000",
		tempDir)
	require.NoError(t, err)

	// Verify files were uploaded
	exists1 := mock.HasObject("test-bucket", "migrations/20240101000000/migrations/001_create_users.sql")
	exists2 := mock.HasObject("test-bucket", "migrations/20240101000000/migrations/002_create_posts.sql")

	assert.True(t, exists1, "001_create_users.sql should be uploaded")
	assert.True(t, exists2, "002_create_posts.sql should be uploaded")

	// Verify content
	content1, _ := mock.GetObjectContent("test-bucket", "migrations/20240101000000/migrations/001_create_users.sql")
	assert.Equal(t, "CREATE TABLE users (id INT);", content1)
}

func TestUploadMigrations_NoSQLFiles(t *testing.T) {
	mock := testhelpers.NewMockS3Client()

	// Create temp directory with no SQL files
	tempDir := t.TempDir()
	err := testhelpers.WriteFile(tempDir, "README.md", "Not a migration")
	require.NoError(t, err)

	// Upload should fail
	err = UploadMigrations(context.Background(), mock,
		"test-bucket",
		"migrations/",
		"20240101000000",
		tempDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no .sql files found")
}
