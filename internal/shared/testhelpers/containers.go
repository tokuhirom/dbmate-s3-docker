package testhelpers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/lib/pq"
)

// TestEnvironment holds all test infrastructure for integration tests
type TestEnvironment struct {
	PostgresContainer testcontainers.Container
	FakeS3Server      *httptest.Server
	DatabaseURL       string
	S3Client          *s3.Client
	S3Bucket          string
	S3EndpointURL     string
	DB                *sql.DB
	t                 *testing.T
}

// SetupPostgresContainer starts a PostgreSQL container and returns the connection string
func SetupPostgresContainer(ctx context.Context, t *testing.T) (testcontainers.Container, string) {
	t.Helper()

	postgresContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err, "Failed to start PostgreSQL container")

	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "Failed to get PostgreSQL connection string")

	return postgresContainer, connStr
}

// SetupFakeS3 starts an in-memory fake S3 server for testing
func SetupFakeS3(ctx context.Context, t *testing.T) (*httptest.Server, string, *s3.Client) {
	t.Helper()

	// Create in-memory S3 backend
	backend := s3mem.New()
	faker := gofakes3.New(backend)
	ts := httptest.NewServer(faker.Server())

	// Create S3 client configured for fake S3
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			"test",
			"test",
			"",
		)),
	)
	require.NoError(t, err, "Failed to create AWS config")

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String(ts.URL)
	})

	return ts, ts.URL, s3Client
}

// SetupTestEnvironment creates a complete test environment with PostgreSQL and fake S3
func SetupTestEnvironment(ctx context.Context, t *testing.T) *TestEnvironment {
	t.Helper()

	// Start PostgreSQL
	postgresContainer, dbURL := SetupPostgresContainer(ctx, t)

	// Start fake S3 server
	fakeS3Server, endpoint, s3Client := SetupFakeS3(ctx, t)

	// Open database connection
	db, err := sql.Open("postgres", dbURL)
	require.NoError(t, err, "Failed to open database connection")

	// Verify database connection
	err = db.Ping()
	require.NoError(t, err, "Failed to ping database")

	// Create test bucket
	bucketName := "test-migrations"
	_, err = s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	require.NoError(t, err, "Failed to create test S3 bucket")

	env := &TestEnvironment{
		PostgresContainer: postgresContainer,
		FakeS3Server:      fakeS3Server,
		DatabaseURL:       dbURL,
		S3Client:          s3Client,
		S3Bucket:          bucketName,
		S3EndpointURL:     endpoint,
		DB:                db,
		t:                 t,
	}

	// Register cleanup
	t.Cleanup(func() {
		env.Cleanup(ctx)
	})

	return env
}

// Cleanup terminates all containers and closes connections
func (e *TestEnvironment) Cleanup(ctx context.Context) {
	if e.DB != nil {
		_ = e.DB.Close()
	}
	if e.PostgresContainer != nil {
		_ = e.PostgresContainer.Terminate(ctx)
	}
	if e.FakeS3Server != nil {
		e.FakeS3Server.Close()
	}
}

// Reset clears all test data for reuse between tests
func (e *TestEnvironment) Reset(ctx context.Context) {
	e.t.Helper()

	// Clear all S3 objects
	e.ClearS3Bucket(ctx)

	// Drop all tables from database
	e.ClearDatabase(ctx)
}

// ClearS3Bucket removes all objects from the test bucket
func (e *TestEnvironment) ClearS3Bucket(ctx context.Context) {
	e.t.Helper()

	// List all objects
	listOutput, err := e.S3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(e.S3Bucket),
	})
	require.NoError(e.t, err, "Failed to list S3 objects")

	// Delete each object
	for _, obj := range listOutput.Contents {
		_, err := e.S3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(e.S3Bucket),
			Key:    obj.Key,
		})
		require.NoError(e.t, err, "Failed to delete S3 object")
	}
}

// ClearDatabase drops all tables from the test database
func (e *TestEnvironment) ClearDatabase(ctx context.Context) {
	e.t.Helper()

	// Drop schema_migrations table if it exists
	_, err := e.DB.ExecContext(ctx, "DROP TABLE IF EXISTS schema_migrations CASCADE")
	require.NoError(e.t, err, "Failed to drop schema_migrations table")

	// Get all user tables
	rows, err := e.DB.QueryContext(ctx, `
		SELECT tablename FROM pg_tables
		WHERE schemaname = 'public'
	`)
	require.NoError(e.t, err, "Failed to query tables")
	defer func() { _ = rows.Close() }()

	var tables []string
	for rows.Next() {
		var tableName string
		err := rows.Scan(&tableName)
		require.NoError(e.t, err, "Failed to scan table name")
		tables = append(tables, tableName)
	}

	// Drop each table
	for _, table := range tables {
		_, err := e.DB.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table))
		require.NoError(e.t, err, "Failed to drop table %s", table)
	}
}

// UploadMigration uploads a single migration file to S3 with the correct path structure
// Path: migrations/<version>/migrations/<filename>.sql
func (e *TestEnvironment) UploadMigration(ctx context.Context, version, filename, sqlContent string) {
	e.t.Helper()

	key := fmt.Sprintf("migrations/%s/migrations/%s", version, filename)
	_, err := e.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(e.S3Bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader([]byte(sqlContent)),
	})
	require.NoError(e.t, err, "Failed to upload migration to S3")
}

// UploadMigrationFile uploads a migration file from filesystem to S3
func (e *TestEnvironment) UploadMigrationFile(ctx context.Context, version, filePath string) {
	e.t.Helper()

	content, err := os.ReadFile(filePath)
	require.NoError(e.t, err, "Failed to read migration file")

	filename := filepath.Base(filePath)
	e.UploadMigration(ctx, version, filename, string(content))
}

// UploadMigrationsFromDir uploads all migration files from a directory to S3 under a specific version
func (e *TestEnvironment) UploadMigrationsFromDir(ctx context.Context, version, dirPath string) {
	e.t.Helper()

	files, err := os.ReadDir(dirPath)
	require.NoError(e.t, err, "Failed to read migrations directory")

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".sql" {
			continue
		}

		fullPath := filepath.Join(dirPath, file.Name())
		e.UploadMigrationFile(ctx, version, fullPath)
	}
}

// UploadResult uploads a result.json file to S3
// Path: migrations/<version>/result.json
func (e *TestEnvironment) UploadResult(ctx context.Context, version, resultJSON string) {
	e.t.Helper()

	key := fmt.Sprintf("migrations/%s/result.json", version)
	_, err := e.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(e.S3Bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader([]byte(resultJSON)),
	})
	require.NoError(e.t, err, "Failed to upload result to S3")
}

// GetResult retrieves and parses a result.json from S3
// Path: migrations/<version>/result.json
func (e *TestEnvironment) GetResult(ctx context.Context, version string) map[string]interface{} {
	e.t.Helper()

	key := fmt.Sprintf("migrations/%s/result.json", version)
	output, err := e.S3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(e.S3Bucket),
		Key:    aws.String(key),
	})
	require.NoError(e.t, err, "Failed to get result from S3")
	defer func() { _ = output.Body.Close() }()

	var result map[string]interface{}
	err = json.NewDecoder(output.Body).Decode(&result)
	require.NoError(e.t, err, "Failed to decode result JSON")

	return result
}

// ResultExists checks if a result file exists in S3
// Path: migrations/<version>/result.json
func (e *TestEnvironment) ResultExists(ctx context.Context, version string) bool {
	e.t.Helper()

	key := fmt.Sprintf("migrations/%s/result.json", version)
	_, err := e.S3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(e.S3Bucket),
		Key:    aws.String(key),
	})
	return err == nil
}

// AssertTableExists verifies that a table exists in the database
func (e *TestEnvironment) AssertTableExists(t *testing.T, tableName string) {
	t.Helper()

	var exists bool
	err := e.DB.QueryRow(`
		SELECT EXISTS (
			SELECT FROM pg_tables
			WHERE schemaname = 'public'
			AND tablename = $1
		)
	`, tableName).Scan(&exists)
	require.NoError(t, err, "Failed to check if table exists")
	require.True(t, exists, "Table %s should exist", tableName)
}

// AssertTableNotExists verifies that a table does not exist in the database
func (e *TestEnvironment) AssertTableNotExists(t *testing.T, tableName string) {
	t.Helper()

	var exists bool
	err := e.DB.QueryRow(`
		SELECT EXISTS (
			SELECT FROM pg_tables
			WHERE schemaname = 'public'
			AND tablename = $1
		)
	`, tableName).Scan(&exists)
	require.NoError(t, err, "Failed to check if table exists")
	require.False(t, exists, "Table %s should not exist", tableName)
}

// GetAppliedMigrations returns the list of applied migrations from schema_migrations
func (e *TestEnvironment) GetAppliedMigrations(ctx context.Context) []string {
	e.t.Helper()

	rows, err := e.DB.QueryContext(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		// Table might not exist yet
		return []string{}
	}
	defer func() { _ = rows.Close() }()

	var versions []string
	for rows.Next() {
		var version string
		err := rows.Scan(&version)
		require.NoError(e.t, err, "Failed to scan migration version")
		versions = append(versions, version)
	}

	return versions
}
